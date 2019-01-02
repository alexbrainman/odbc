// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexbrainman/odbc/api"
)

var (
	mssrv    = flag.String("mssrv", "server", "ms sql server name")
	msdb     = flag.String("msdb", "dbname", "ms sql server database name")
	msuser   = flag.String("msuser", "", "ms sql server user name")
	mspass   = flag.String("mspass", "", "ms sql server password")
	msdriver = flag.String("msdriver", defaultDriver(), "ms sql odbc driver name")
	msport   = flag.String("msport", "1433", "ms sql server port number")
)

func defaultDriver() string {
	if runtime.GOOS == "windows" {
		return "sql server"
	} else {
		return "freetds"
	}
}

func isFreeTDS() bool {
	return *msdriver == "freetds"
}

type connParams map[string]string

func newConnParams() connParams {
	params := connParams{
		"driver":   *msdriver,
		"server":   *mssrv,
		"database": *msdb,
	}
	if isFreeTDS() {
		params["uid"] = *msuser
		params["pwd"] = *mspass
		params["port"] = *msport
		params["TDS_Version"] = "8.0"
		//params["clientcharset"] = "UTF-8"
		//params["debugflags"] = "0xffff"
	} else {
		if len(*msuser) == 0 {
			params["trusted_connection"] = "yes"
		} else {
			params["uid"] = *msuser
			params["pwd"] = *mspass
		}
	}
	a := strings.SplitN(params["server"], ",", -1)
	if len(a) == 2 {
		params["server"] = a[0]
		params["port"] = a[1]
	}
	return params
}

func (params connParams) getConnAddress() (string, error) {
	port, ok := params["port"]
	if !ok {
		return "", errors.New("no port number provided.")
	}
	host, ok := params["server"]
	if !ok {
		return "", errors.New("no host name provided.")
	}
	return host + ":" + port, nil
}

func (params connParams) updateConnAddress(address string) error {
	a := strings.SplitN(address, ":", -1)
	if len(a) != 2 {
		return fmt.Errorf("listen address must have 2 fields, but %d found", len(a))
	}
	params["server"] = a[0]
	params["port"] = a[1]
	return nil
}

func (params connParams) makeODBCConnectionString() string {
	if port, ok := params["port"]; ok {
		params["server"] += "," + port
		delete(params, "port")
	}
	var c string
	for n, v := range params {
		c += n + "=" + v + ";"
	}
	return c
}

func mssqlConnectWithParams(params connParams) (db *sql.DB, stmtCount int, err error) {
	db, err = sql.Open("odbc", params.makeODBCConnectionString())
	if err != nil {
		return nil, 0, err
	}
	stats := db.Driver().(*Driver).Stats
	return db, stats.StmtCount, nil
}

func mssqlConnect() (db *sql.DB, stmtCount int, err error) {
	return mssqlConnectWithParams(newConnParams())
}

func closeDB(t *testing.T, db *sql.DB, shouldStmtCount, ignoreIfStmtCount int) {
	s := db.Driver().(*Driver).Stats
	err := db.Close()
	if err != nil {
		t.Fatalf("error closing DB: %v", err)
	}
	switch s.StmtCount {
	case shouldStmtCount:
		// all good
	case ignoreIfStmtCount:
		t.Logf("ignoring unexpected StmtCount of %v", ignoreIfStmtCount)
	default:
		t.Errorf("unexpected StmtCount: should=%v, is=%v", ignoreIfStmtCount, s.StmtCount)
	}
}

// as per http://www.mssqltips.com/sqlservertip/2198/determine-which-version-of-sql-server-data-access-driver-is-used-by-an-application/
func connProtoVersion(db *sql.DB) ([]byte, error) {
	var p []byte
	if err := db.QueryRow("select cast(protocol_version as binary(4)) from master.sys.dm_exec_connections where session_id = @@spid").Scan(&p); err != nil {
		return nil, err
	}
	if len(p) != 4 {
		return nil, errors.New("failed to fetch connection protocol")
	}
	return p, nil
}

// as per http://msdn.microsoft.com/en-us/library/dd339982.aspx
func isProto2008OrLater(db *sql.DB) (bool, error) {
	p, err := connProtoVersion(db)
	if err != nil {
		return false, err
	}
	return p[0] >= 0x73, nil
}

// as per http://www.mssqltips.com/sqlservertip/2563/understanding-the-sql-server-select-version-command/
func serverVersion(db *sql.DB) (sqlVersion, sqlPartNumber, osVersion string, err error) {
	var v string
	if err = db.QueryRow("select @@version").Scan(&v); err != nil {
		return "", "", "", err
	}
	a := strings.SplitN(v, "\n", -1)
	if len(a) < 4 {
		return "", "", "", errors.New("SQL Server version string must have at least 4 lines: " + v)
	}
	for i := range a {
		a[i] = strings.Trim(a[i], " \t")
	}
	l1 := strings.SplitN(a[0], "- ", -1)
	if len(l1) != 2 {
		return "", "", "", errors.New("SQL Server version first line must have - in it: " + v)
	}
	i := strings.Index(a[3], " on ")
	if i < 0 {
		return "", "", "", errors.New("SQL Server version fourth line must have 'on' in it: " + v)
	}
	sqlVersion = l1[0] + a[3][:i]
	osVersion = a[3][i+4:]
	sqlPartNumber = strings.Trim(l1[1], " ")
	l12 := strings.SplitN(sqlPartNumber, " ", -1)
	if len(l12) < 2 {
		return "", "", "", errors.New("SQL Server version first line must have space after part number in it: " + v)
	}
	sqlPartNumber = l12[0]
	return sqlVersion, sqlPartNumber, osVersion, nil
}

// as per http://www.mssqltips.com/sqlservertip/2563/understanding-the-sql-server-select-version-command/
func isSrv2008OrLater(db *sql.DB) (bool, error) {
	_, sqlPartNumber, _, err := serverVersion(db)
	if err != nil {
		return false, err
	}
	a := strings.SplitN(sqlPartNumber, ".", -1)
	if len(a) != 4 {
		return false, errors.New("SQL Server part number must have 4 numbers in it: " + sqlPartNumber)
	}
	n, err := strconv.ParseInt(a[0], 10, 0)
	if err != nil {
		return false, errors.New("SQL Server invalid part number: " + sqlPartNumber)
	}
	return n >= 10, nil
}

func is2008OrLater(db *sql.DB) bool {
	b, err := isSrv2008OrLater(db)
	if err != nil || !b {
		return false
	}
	b, err = isProto2008OrLater(db)
	if err != nil || !b {
		return false
	}
	return true
}

func exec(t *testing.T, db *sql.DB, query string) {
	// TODO(brainman): make sure https://github.com/golang/go/issues/3678 is fixed
	//r, err := db.Exec(query, a...)
	s, err := db.Prepare(query)
	if err != nil {
		t.Fatalf("db.Prepare(%q) failed: %v", query, err)
	}
	defer s.Close()
	r, err := s.Exec()
	if err != nil {
		t.Fatalf("s.Exec(%q ...) failed: %v", query, err)
	}
	_, err = r.RowsAffected()
	if err != nil {
		t.Fatalf("r.RowsAffected(%q ...) failed: %v", query, err)
	}
}

func driverExec(t *testing.T, dc driver.Conn, query string) {
	st, err := dc.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil && t != nil {
			t.Fatal(err)
		}
	}()

	r, err := st.Exec([]driver.Value{})
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		return
	}
	_, err = r.RowsAffected()
	if err != nil {
		if t != nil {
			t.Fatalf("r.RowsAffected(%q ...) failed: %v", query, err)
		}
		return
	}
}

func TestMSSQLCreateInsertDelete(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	type friend struct {
		age       int
		isGirl    bool
		weight    float64
		dob       time.Time
		data      []byte
		canBeNull sql.NullString
	}
	var friends = map[string]friend{
		"glenda": {
			age:       5,
			isGirl:    true,
			weight:    15.5,
			dob:       time.Date(2000, 5, 10, 11, 1, 1, 0, time.Local),
			data:      []byte{0x0, 0x0, 0xb, 0xad, 0xc0, 0xde},
			canBeNull: sql.NullString{"aa", true},
		},
		"gopher": {
			age:       3,
			isGirl:    false,
			weight:    26.12,
			dob:       time.Date(2009, 5, 10, 11, 1, 1, 123e6, time.Local),
			data:      []byte{0x0},
			canBeNull: sql.NullString{"bbb", true},
		},
	}

	// create table
	db.Exec("drop table dbo.temp")
	exec(t, db, "create table dbo.temp (name varchar(20), age int, isGirl bit, weight decimal(5,2), dob datetime, data varbinary(10) null, canBeNull varchar(10) null)")
	func() {
		s, err := db.Prepare("insert into dbo.temp (name, age, isGirl, weight, dob, data, canBeNull) values (?, ?, ?, ?, ?, cast(? as varbinary(10)), ?)")
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		for name, f := range friends {
			_, err := s.Exec(name, f.age, f.isGirl, f.weight, f.dob, f.data, f.canBeNull)
			if err != nil {
				t.Fatal(err)
			}
		}
		_, err = s.Exec("chris", 25, 0, 50, time.Date(2015, 12, 25, 0, 0, 0, 0, time.Local), "ccc", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Exec("null", 0, 0, 0, time.Date(2015, 12, 25, 1, 2, 3, 0, time.Local), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// read from the table and verify returned results
	rows, err := db.Query("select name, age, isGirl, weight, dob, data, canBeNull from dbo.temp")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var is friend
		err = rows.Scan(&name, &is.age, &is.isGirl, &is.weight, &is.dob, &is.data, &is.canBeNull)
		if err != nil {
			t.Fatal(err)
		}
		want, ok := friends[name]
		if !ok {
			switch name {
			case "chris":
				// we know about chris, we just do not like him
			case "null":
				if is.canBeNull.Valid {
					t.Errorf("null's canBeNull is suppose to be NULL, but is %v", is.canBeNull)
				}
			default:
				t.Errorf("found %s, who is not my friend", name)
			}
			continue
		}
		if is.age < want.age {
			t.Errorf("I did not know, that %s is so young (%d, but %d expected)", name, is.age, want.age)
			continue
		}
		if is.age > want.age {
			t.Errorf("I did not know, that %s is so old (%d, but %d expected)", name, is.age, want.age)
			continue
		}
		if is.isGirl != want.isGirl {
			if is.isGirl {
				t.Errorf("I did not know, that %s is a girl", name)
			} else {
				t.Errorf("I did not know, that %s is a boy", name)
			}
			continue
		}
		if is.weight != want.weight {
			t.Errorf("I did not know, that %s weighs %fkg (%fkg expected)", name, is.weight, want.weight)
			continue
		}
		if !is.dob.Equal(want.dob) {
			t.Errorf("I did not know, that %s's date of birth is %v (%v expected)", name, is.dob, want.dob)
			continue
		}
		if !bytes.Equal(is.data, want.data) {
			t.Errorf("I did not know, that %s's data is %v (%v expected)", name, is.data, want.data)
			continue
		}
		if is.canBeNull != want.canBeNull {
			t.Errorf("canBeNull for %s is wrong (%v, but %v expected)", name, is.canBeNull, want.canBeNull)
			continue
		}
	}
	err = rows.Err()
	if err != nil {
		t.Fatal(err)
	}

	// clean after ourselves
	exec(t, db, "drop table dbo.temp")
}

func TestMSSQLTransactions(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, "create table dbo.temp (name varchar(20))")

	var was, is int
	err = db.QueryRow("select count(*) from dbo.temp").Scan(&was)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec("insert into dbo.temp (name) values ('tx1')")
	if err != nil {
		t.Fatal(err)
	}
	err = tx.QueryRow("select count(*) from dbo.temp").Scan(&is)
	if err != nil {
		t.Fatal(err)
	}
	if was+1 != is {
		t.Fatalf("is(%d) should be 1 more then was(%d)", is, was)
	}
	ch := make(chan error)
	go func() {
		// this will block until our transaction is finished
		err = db.QueryRow("select count(*) from dbo.temp").Scan(&is)
		if err != nil {
			ch <- err
		}
		if was+1 != is {
			ch <- fmt.Errorf("is(%d) should be 1 more then was(%d)", is, was)
		}
		ch <- nil
	}()
	time.Sleep(100 * time.Millisecond)
	tx.Commit()
	err = <-ch
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow("select count(*) from dbo.temp").Scan(&is)
	if err != nil {
		t.Fatal(err)
	}
	if was+1 != is {
		t.Fatalf("is(%d) should be 1 more then was(%d)", is, was)
	}

	was = is
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec("insert into dbo.temp (name) values ('tx2')")
	if err != nil {
		t.Fatal(err)
	}
	err = tx.QueryRow("select count(*) from dbo.temp").Scan(&is)
	if err != nil {
		t.Fatal(err)
	}
	if was+1 != is {
		t.Fatalf("is(%d) should be 1 more then was(%d)", is, was)
	}
	tx.Rollback()
	err = db.QueryRow("select count(*) from dbo.temp").Scan(&is)
	if err != nil {
		t.Fatal(err)
	}
	if was != is {
		t.Fatalf("is(%d) should be equal to was(%d)", is, was)
	}

	exec(t, db, "drop table dbo.temp")
}

type matchFunc func(v interface{}) error

func match(a interface{}) matchFunc {
	return func(b interface{}) error {
		switch got := b.(type) {
		case nil:
			switch expect := a.(type) {
			case nil:
				// matching
			default:
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case bool:
			expect, ok := a.(bool)
			if !ok {
				return fmt.Errorf("couldn't convert expected value %v(%T) to %T", a, a, got)
			}
			if got != expect {
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case int32:
			expect, ok := a.(int32)
			if !ok {
				return fmt.Errorf("couldn't convert expected value %v(%T) to %T", a, a, got)
			}
			if got != expect {
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case int64:
			expect, ok := a.(int64)
			if !ok {
				return fmt.Errorf("couldn't convert expected value %v(%T) to %T", a, a, got)
			}
			if got != expect {
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case float64:
			switch expect := a.(type) {
			case float64:
				if got != expect {
					return fmt.Errorf("expect %v, but got %v", expect, got)
				}
			case int64:
				if got != float64(expect) {
					return fmt.Errorf("expect %v, but got %v", expect, got)
				}
			default:
				return fmt.Errorf("unsupported type %T", expect)
			}
		case string:
			expect, ok := a.(string)
			if !ok {
				return fmt.Errorf("couldn't convert expected value %v(%T) to %T", a, a, got)
			}
			if got != expect {
				return fmt.Errorf("expect %q, but got %q", expect, got)
			}
		case []byte:
			expect, ok := a.([]byte)
			if !ok {
				return fmt.Errorf("couldn't convert expected value %v(%T) to %T", a, a, got)
			}
			if !bytes.Equal(got, expect) {
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case time.Time:
			expect, ok := a.(time.Time)
			if !ok {
				return fmt.Errorf("couldn't convert expected value %v(%T) to %T", a, a, got)
			}
			if !got.Equal(expect) {
				return fmt.Errorf("expect %q, but got %q", expect, got)
			}
		default:
			return fmt.Errorf("unsupported type %T", got)
		}
		return nil
	}
}

type typeTest struct {
	query string
	match matchFunc
}

var veryLongString = strings.Repeat("abcd ", 206)

var typeTests = []typeTest{
	// bool
	{"select cast(1 as bit)", match(true)},
	{"select cast(2 as bit)", match(true)},
	{"select cast(0 as bit)", match(false)},
	{"select cast(NULL as bit)", match(nil)},

	// int
	{"select cast(0 as int)", match(int32(0))},
	{"select cast(123 as int)", match(int32(123))},
	{"select cast(-4 as int)", match(int32(-4))},
	{"select cast(NULL as int)", match(nil)},
	{"select cast(0 as tinyint)", match(int32(0))},
	{"select cast(255 as tinyint)", match(int32(255))},
	{"select cast(-32768 as smallint)", match(int32(-32768))},
	{"select cast(32767 as smallint)", match(int32(32767))},
	{"select cast(-9223372036854775808 as bigint)", match(int64(-9223372036854775808))},
	{"select cast(9223372036854775807 as bigint)", match(int64(9223372036854775807))},

	// decimal, float, real
	{"select cast(123 as decimal(5, 0))", match(float64(123))},
	{"select cast(-123 as decimal(5, 0))", match(float64(-123))},
	{"select cast(123.5 as decimal(5, 0))", match(float64(124))},
	{"select cast(NULL as decimal(5, 0))", match(nil)},
	{"select cast(123.45 as decimal(5, 2))", match(123.45)},
	{"select cast(-123.45 as decimal(5, 2))", match(-123.45)},
	{"select cast(123.456 as decimal(5, 2))", match(123.46)},
	{"select cast(0.123456789 as float)", match(0.123456789)},
	{"select cast(NULL as float)", match(nil)},
	{"select cast(3.6666667461395264 as real)", match(3.6666667461395264)},
	{"select cast(NULL as real)", match(nil)},
	{"select cast(1.2333333504e+10 as real)", match(1.2333333504e+10)},

	// money
	{"select cast(12 as money)", match(float64(12))},
	{"select cast(-12 as money)", match(float64(-12))},
	{"select cast(0.01 as money)", match(0.01)},
	{"select cast(0.0123 as money)", match(0.0123)},
	{"select cast(NULL as money)", match(nil)},
	{"select cast(1 as smallmoney)", match(float64(1))},
	{"select cast(0.0123 as smallmoney)", match(0.0123)},
	{"select cast(NULL as smallmoney)", match(nil)},

	// strings
	{"select cast(123 as varchar(21))", match([]byte("123"))},
	{"select cast(123 as char(5))", match([]byte("123  "))},
	{"select cast('abcde' as varchar(3))", match([]byte("abc"))},
	{"select cast('' as varchar(5))", match([]byte(""))},
	{"select cast(NULL as varchar(5))", match(nil)},
	{"select cast(123 as nvarchar(21))", match([]byte("123"))},
	{"select cast('abcde' as nvarchar(3))", match([]byte("abc"))},
	{"select cast('' as nvarchar(5))", match([]byte(""))},
	{"select cast(NULL as nvarchar(5))", match(nil)},

	// datetime, smalldatetime
	{"select cast('20151225' as datetime)", match(time.Date(2015, 12, 25, 0, 0, 0, 0, time.Local))},
	{"select cast('2007-05-08 12:35:29.123' as datetime)", match(time.Date(2007, 5, 8, 12, 35, 29, 123e6, time.Local))},
	{"select cast(NULL as datetime)", match(nil)},
	{"select cast('2007-05-08 12:35:29.123' as smalldatetime)", match(time.Date(2007, 5, 8, 12, 35, 0, 0, time.Local))},

	// uniqueidentifier
	{"select cast('0e984725-c51c-4bf4-9960-e1c80e27aba0' as uniqueidentifier)", match("0e984725-c51c-4bf4-9960-e1c80e27aba0")},
	{"select cast(NULL as uniqueidentifier)", match(nil)},

	// string blobs
	{"select cast('abc' as varchar(max))", match([]byte("abc"))},
	{"select cast('' as varchar(max))", match([]byte(""))},
	{fmt.Sprintf("select cast('%s' as varchar(max))", veryLongString), match([]byte(veryLongString))},
	{"select cast(NULL as varchar(max))", match(nil)},
	{"select cast('abc' as nvarchar(max))", match([]byte("abc"))},
	{"select cast('' as nvarchar(max))", match([]byte(""))},
	{fmt.Sprintf("select cast('%s' as nvarchar(max))", veryLongString), match([]byte(veryLongString))},
	{"select cast(NULL as nvarchar(max))", match(nil)},
	{"select cast('abc' as text)", match([]byte("abc"))},
	{"select cast('' as text)", match([]byte(""))},
	{fmt.Sprintf("select cast('%s' as text)", veryLongString), match([]byte(veryLongString))},
	{"select cast(NULL as text)", match(nil)},
	{"select cast('abc' as ntext)", match([]byte("abc"))},
	{"select cast('' as ntext)", match([]byte(""))},
	{fmt.Sprintf("select cast('%s' as ntext)", veryLongString), match([]byte(veryLongString))},
	{"select cast(NULL as ntext)", match(nil)},

	// xml
	{"select cast(N'<root>hello</root>' as xml)", match([]byte("<root>hello</root>"))},
	{"select cast(N'<root><doc><item1>dd</item1></doc></root>' as xml)", match([]byte("<root><doc><item1>dd</item1></doc></root>"))},

	// binary blobs
	{"select cast('abc' as binary(5))", match([]byte{'a', 'b', 'c', 0, 0})},
	{"select cast('' as binary(5))", match([]byte{0, 0, 0, 0, 0})},
	{"select cast(NULL as binary(5))", match(nil)},
	{"select cast('abc' as varbinary(5))", match([]byte{'a', 'b', 'c'})},
	{"select cast('' as varbinary(5))", match([]byte(""))},
	{"select cast(NULL as varbinary(5))", match(nil)},
	{"select cast('abc' as varbinary(max))", match([]byte{'a', 'b', 'c'})},
	{"select cast('' as varbinary(max))", match([]byte(""))},
	{fmt.Sprintf("select cast('%s' as varbinary(max))", veryLongString), match([]byte(veryLongString))},
	{"select cast(NULL as varbinary(max))", match(nil)},
}

// TODO(brainman): see why typeMSSpecificTests do not work on freetds

var typeMSSpecificTests = []typeTest{
	{"select cast(N'\u0421\u0430\u0448\u0430' as nvarchar(5))", match([]byte("\u0421\u0430\u0448\u0430"))},
	{"select cast(N'\u0421\u0430\u0448\u0430' as nvarchar(max))", match([]byte("\u0421\u0430\u0448\u0430"))},
	{"select cast(N'\u0421\u0430\u0448\u0430' as ntext)", match([]byte("\u0421\u0430\u0448\u0430"))},
}

var typeMSSQL2008Tests = []typeTest{
	// datetime2
	{"select cast('20151225' as datetime2)", match(time.Date(2015, 12, 25, 0, 0, 0, 0, time.Local))},
	{"select cast('2007-05-08 12:35:29.1234567' as datetime2)", match(time.Date(2007, 5, 8, 12, 35, 29, 1234567e2, time.Local))},
	{"select cast(NULL as datetime2)", match(nil)},

	// time(7)
	{"select cast('12:35:29.1234567' as time(7))", match(time.Date(1, 1, 1, 12, 35, 29, 1234567e2, time.Local))},
	{"select cast(NULL as time(7))", match(nil)},
}

var typeTestsToFail = []string{
	// int
	"select cast(-1 as tinyint)",
	"select cast(256 as tinyint)",
	"select cast(-32769 as smallint)",
	"select cast(32768 as smallint)",
	"select cast(-9223372036854775809 as bigint)",
	"select cast(9223372036854775808 as bigint)",

	// decimal
	"select cast(1234.5 as decimal(5, 2))",

	// uniqueidentifier
	"select cast('0x984725-c51c-4bf4-9960-e1c80e27aba0' as uniqueidentifier)",
}

func TestMSSQLTypes(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	tests := typeTests
	if !isFreeTDS() {
		tests = append(tests, typeMSSpecificTests...)
	}
	if is2008OrLater(db) {
		tests = append(tests, typeMSSQL2008Tests...)
	}
	for _, r := range tests {
		func() {
			rows, err := db.Query(r.query)
			if err != nil {
				t.Errorf("db.Query(%q) failed: %v", r.query, err)
				return
			}
			defer rows.Close()
			for rows.Next() {
				var got interface{}
				err := rows.Scan(&got)
				if err != nil {
					t.Errorf("rows.Scan for %q failed: %v", r.query, err)
					return
				}
				err = r.match(got)
				if err != nil {
					t.Errorf("test %q failed: %v", r.query, err)
				}
			}
			err = rows.Err()
			if err != nil {
				t.Error(err)
				return
			}
		}()
	}

	for _, query := range typeTestsToFail {
		rows, err := db.Query(query)
		if err != nil {
			continue
		}
		rows.Close()
		t.Errorf("test %q passed, but should fail", query)
	}
}

// TestMSSQLIntAfterText verify that non-bindable column can
// precede bindable column.
func TestMSSQLIntAfterText(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	const query = "select cast('abc' as text), cast(123 as int)"
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("db.Query(%q) failed: %v", query, err)
	}
	defer rows.Close()
	for rows.Next() {
		var i int
		var text string
		err = rows.Scan(&text, &i)
		if err != nil {
			t.Fatalf("rows.Scan for %q failed: %v", query, err)
		}
		if text != "abc" {
			t.Errorf("expected \"abc\", but received %v", text)
		}
		if i != 123 {
			t.Errorf("expected 123, but received %v", i)
		}
	}
	err = rows.Err()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMSSQLStmtAndRows(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// not checking resources usage here, because these are
		// unpredictable due to use of goroutines.
		err := db.Close()
		if err != nil {
			t.Fatalf("error closing DB: %v", err)
		}
	}()

	var staff = map[string][]string{
		"acc": {"John", "Mary", "Moe"},
		"eng": {"Bar", "Foo", "Uno"},
		"sls": {"Scrudge", "Sls2", "Sls3"},
	}

	db.Exec("drop table dbo.temp")
	exec(t, db, "create table dbo.temp (dept char(3), name varchar(20))")

	func() {
		// test 1 Stmt and many Exec's
		s, err := db.Prepare("insert into dbo.temp (dept, name) values (?, ?)")
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		for dept, people := range staff {
			for _, person := range people {
				_, err := s.Exec(dept, person)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}()

	func() {
		// test Stmt is closed before Rows are
		s, err := db.Prepare("select name from dbo.temp")
		if err != nil {
			t.Fatal(err)
		}

		r, err := s.Query()
		if err != nil {
			s.Close()
			t.Fatal(err)
		}
		defer r.Close()

		// TODO(brainman): dangling statement(s) bug reported
		// https://github.com/golang/go/issues/3865
		err = s.Close()
		if err != nil {
			t.Fatal(err)
		}

		n := 0
		for r.Next() {
			var name string
			err = r.Scan(&name)
			if err != nil {
				t.Fatal(err)
			}
			n++
		}
		err = r.Err()
		if err != nil {
			t.Fatal(err)
		}
		const should = 9
		if n != should {
			t.Fatalf("expected %v, but received %v", should, n)
		}
	}()

	if db.Driver().(*Driver).Stats.StmtCount != sc {
		t.Fatalf("invalid statement count: expected %v, is %v", sc, db.Driver().(*Driver).Stats.StmtCount)
	}

	// no resource tracking past this point

	func() {
		// test 1 Stmt and many Query's executed one after the other
		s, err := db.Prepare("select name from dbo.temp where dept = ? order by name")
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		for dept, people := range staff {
			func() {
				r, err := s.Query(dept)
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				i := 0
				for r.Next() {
					var is string
					err = r.Scan(&is)
					if err != nil {
						t.Fatal(err)
					}
					if people[i] != is {
						t.Fatalf("expected %v, but received %v", people[i], is)
					}
					i++
				}
				err = r.Err()
				if err != nil {
					t.Fatal(err)
				}
			}()
		}
		// test 1 Stmt and many simultaneous Query's
		eof := fmt.Errorf("eof")
		ch := make(map[string]chan error)
		for dept, people := range staff {
			c := make(chan error)
			go func(c chan error, dept string, people []string) {
				c <- nil
				// NOTE(brainman): this could actually re-prepare since
				// we are running it simultaneously in multiple goroutines
				r, err := s.Query(dept)
				if err != nil {
					c <- fmt.Errorf("%v", err)
					return
				}
				defer r.Close()
				i := 0
				c <- nil
				for r.Next() {
					var is string
					c <- nil
					err = r.Scan(&is)
					if err != nil {
						c <- fmt.Errorf("%v", err)
						return
					}
					c <- nil
					if people[i] != is {
						c <- fmt.Errorf("expected %v, but received %v", people[i], is)
						return
					}
					i++
				}
				err = r.Err()
				if err != nil {
					c <- fmt.Errorf("%v", err)
					return
				}
				c <- eof
			}(c, dept, people)
			ch[dept] = c
		}
		for len(ch) > 0 {
			for dept, c := range ch {
				err := <-c
				if err != nil {
					if err != eof {
						t.Errorf("dept=%v: %v", dept, err)
					}
					delete(ch, dept)
				}
			}
		}
	}()

	exec(t, db, "drop table dbo.temp")
}

func TestMSSQLIssue5(t *testing.T) {
	testingIssue5 = true
	defer func() {
		testingIssue5 = false
	}()
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}

	const nworkers = 8
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, `
		create table dbo.temp (
			id int,
			value int,
			constraint [pk_id] primary key ([id])
		)
	`)

	var count int32

	runCycle := func(waitch <-chan struct{}, errch chan<- error) (reterr error) {
		defer func() {
			errch <- reterr
		}()
		stmt, err := db.Prepare("insert into dbo.temp (id, value) values (?, ?)")
		if err != nil {
			return fmt.Errorf("Prepare failed: %v", err)
		}
		defer stmt.Close()
		errch <- nil
		<-waitch
		for {
			i := (int)(atomic.AddInt32(&count, 1))
			_, err := stmt.Exec(i, i)
			if err != nil {
				return fmt.Errorf("Exec failed i=%d: %v", i, err)
			}
			runtime.GC()
			if i >= 100 {
				break
			}
		}
		return
	}

	waitch := make(chan struct{})
	errch := make(chan error, nworkers)
	for i := 0; i < nworkers; i++ {
		go runCycle(waitch, errch)
	}
	for i := 0; i < nworkers; i++ {
		if err := <-errch; err != nil {
			t.Error(err)
		}
	}
	if t.Failed() {
		return
	}
	close(waitch)
	for i := 0; i < nworkers; i++ {
		if err := <-errch; err != nil {
			t.Fatal(err)
		}
	}
	// TODO: maybe I should verify dbo.temp records here

	exec(t, db, "drop table dbo.temp")
}

func TestMSSQLDeleteNonExistent(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, "create table dbo.temp (name varchar(20))")
	_, err = db.Exec("insert into dbo.temp (name) values ('alex')")
	if err != nil {
		t.Fatal(err)
	}

	r, err := db.Exec("delete from dbo.temp where name = 'bob'")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	cnt, err := r.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected failed: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("RowsAffected returns %d, but 0 expected", cnt)
	}

	exec(t, db, "drop table dbo.temp")
}

// https://github.com/alexbrainman/odbc/issues/14
func TestMSSQLDatetime2Param(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	if !is2008OrLater(db) {
		t.Skip("skipping test; needs MS SQL Server 2008 or later")
	}

	db.Exec("drop table dbo.temp")
	exec(t, db, "create table dbo.temp (dt datetime2)")

	expect := time.Date(2007, 5, 8, 12, 35, 29, 1234567e2, time.Local)
	_, err = db.Exec("insert into dbo.temp (dt) values (?)", expect)
	if err != nil {
		t.Fatal(err)
	}
	var got time.Time
	err = db.QueryRow("select top 1 dt from dbo.temp").Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if expect != got {
		t.Fatalf("expect %v, but got %v", expect, got)
	}

	exec(t, db, "drop table dbo.temp")
}

// https://github.com/alexbrainman/odbc/issues/19
func TestMSSQLMerge(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	if !is2008OrLater(db) {
		t.Skip("skipping test; needs MS SQL Server 2008 or later")
	}

	db.Exec("drop table dbo.temp")
	exec(t, db, `
		create table dbo.temp (
			id int not null,
			name varchar(20),
			constraint pk_temp primary key(id)
		)
	`)
	for i := 0; i < 5; i++ {
		_, err = db.Exec("insert into dbo.temp (id, name) values (?, ?)", i, fmt.Sprintf("gordon%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	s, err := db.Prepare(`
		merge into dbo.temp as dest
		using ( values (?, ?) ) as src (id, name) on src.id = dest.id
		when matched then update set dest.name = src.name
		when not matched then insert values (src.id, src.name);
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var tests = []struct {
		id   int
		name string
	}{
		{id: 1, name: "new name1"},
		{id: 8, name: "hohoho"},
	}
	for _, test := range tests {
		_, err = s.Exec(test.id, test.name)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range tests {
		var got string
		err = db.QueryRow("select name from dbo.temp where id = ?", test.id).Scan(&got)
		if err != nil {
			t.Fatal(err)
		}
		if test.name != got {
			t.Fatalf("expect %v, but got %v", test.name, got)
		}
	}

	exec(t, db, "drop table dbo.temp")
}

// https://github.com/alexbrainman/odbc/issues/20
func TestMSSQLSelectInt(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	const expect = 123456
	var got int
	if err := db.QueryRow("select ?", expect).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if expect != got {
		t.Fatalf("expect %v, but got %v", expect, got)
	}
}

// https://github.com/alexbrainman/odbc/issues/21
func TestMSSQLTextColumnParam(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, `create table dbo.temp(id int primary key not null, v1 text, v2 text, v3 text, v4 text, v5 text, v6 text, v7 text, v8 text)`)

	s, err := db.Prepare(`insert into dbo.temp(id, v1, v2, v3, v4, v5, v6, v7, v8) values (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	b := "string string string string string string string string string"
	for i := 0; i < 100; i++ {
		_, err := s.Exec(i, b, b, b, b, b, b, b, b)
		if err != nil {
			t.Fatal(err)
		}
	}

	exec(t, db, "drop table dbo.temp")
}

func digestString(s string) string {
	if len(s) < 40 {
		return s
	}
	return fmt.Sprintf("%s ... (%d bytes long)", s[:40], len(s))
}

func digestBytes(b []byte) string {
	if len(b) < 20 {
		return fmt.Sprintf("%v", b)
	}
	s := ""
	for _, v := range b[:20] {
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("[%v ...] (%d bytes long)", s, len(b))
}

var paramTypeTests = []struct {
	description string
	sqlType     string
	value       interface{}
}{
	// nil parameters
	{"NULL for bit", "bit", nil},
	{"NULL for text", "text", nil},
	{"NULL for int", "int", nil},
	// strings
	{"non empty string", "varchar(10)", "abc"},
	{"one character string", "varchar(10)", "a"},
	{"empty string", "varchar(10)", ""},
	{"empty unicode string", "nvarchar(10)", ""},
	{"3999 large unicode string", "nvarchar(max)", strings.Repeat("a", 3999)},
	{"4000 large unicode string", "nvarchar(max)", strings.Repeat("a", 4000)},
	{"4000 large non-ascii unicode string", "nvarchar(max)", strings.Repeat("\u0421", 4000)},
	{"4001 large unicode string", "nvarchar(max)", strings.Repeat("a", 4001)},
	{"4001 large non-ascii unicode string", "nvarchar(max)", strings.Repeat("\u0421", 4001)},
	{"10000 large unicode string", "nvarchar(max)", strings.Repeat("a", 10000)},
	{"empty unicode null string", "nvarchar(10) null", ""},
	{"3999 large string value", "text", strings.Repeat("a", 3999)},
	{"4000 large string value", "text", strings.Repeat("a", 4000)},
	{"4000 large unicode string value", "ntext", strings.Repeat("\u0421", 4000)},
	{"4001 large string value", "text", strings.Repeat("a", 4001)},
	{"4001 large unicode string value", "ntext", strings.Repeat("\u0421", 4001)},
	{"very large string value", "text", strings.Repeat("a", 10000)},
	// datetime
	{"datetime overflow", "datetime", time.Date(2013, 9, 9, 14, 07, 15, 123e6, time.Local)},
	// binary blobs
	{"small blob", "varbinary", make([]byte, 1)},
	{"very large blob", "varbinary(max)", make([]byte, 100000)},
	{"7999 large image", "image", make([]byte, 7999)},
	{"8000 large image", "image", make([]byte, 8000)},
	{"8001 large image", "image", make([]byte, 8001)},
	{"very large image", "image", make([]byte, 10000)},
}

func TestMSSQLTextColumnParamTypes(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	for _, test := range paramTypeTests {
		db.Exec("drop table dbo.temp")
		exec(t, db, fmt.Sprintf("create table dbo.temp(v %s)", test.sqlType))
		_, err = db.Exec("insert into dbo.temp(v) values(?)", test.value)
		if err != nil {
			t.Errorf("%s insert test failed: %s", test.description, err)
		}
		var v interface{}
		err = db.QueryRow("select v from dbo.temp").Scan(&v)
		if err != nil {
			t.Errorf("%s select test failed: %s", test.description, err)
			continue
		}
		switch want := test.value.(type) {
		case string:
			have := string(v.([]byte))
			if have != want {
				t.Errorf("%s wrong return value: have %q; want %q", test.description, digestString(have), digestString(want))
			}
		case []byte:
			have := v.([]byte)
			if !bytes.Equal(have, want) {
				t.Errorf("%s wrong return value: have %v; want %v", test.description, digestBytes(have), digestBytes(want))
			}
		case time.Time:
			have := v.(time.Time)
			if have != want {
				t.Errorf("%s wrong return value: have %v; want %v", test.description, have, want)
			}
		case nil:
			if v != nil {
				t.Errorf("%s wrong return value: have %v; want nil", test.description, v)
			}
		}
	}
	exec(t, db, "drop table dbo.temp")
}

func TestMSSQLLongColumnNames(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	query := fmt.Sprintf("select 'hello' as %s", strings.Repeat("a", 110))
	var s string
	err = db.QueryRow(query).Scan(&s)
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Errorf("expected \"hello\", but received %v", s)
	}
}

func TestMSSQLRawBytes(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, `create table dbo.temp(ascii char(7), utf16 nchar(7), blob binary(3))`)
	_, err = db.Exec(`insert into dbo.temp (ascii, utf16, blob) values (?, ?, ?)`, "alex", "alex", []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query("select ascii, utf16, blob from dbo.temp")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ascii, utf16 sql.RawBytes
		var blob []byte
		err = rows.Scan(&ascii, &utf16, &blob)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
	}
	err = rows.Err()
	if err != nil {
		t.Fatal(err)
	}

	exec(t, db, "drop table dbo.temp")
}

// https://github.com/alexbrainman/odbc/issues/27
func TestMSSQLUTF16ToUTF8(t *testing.T) {
	s := []uint16{0x47, 0x75, 0x73, 0x74, 0x61, 0x66, 0x27, 0x73, 0x20, 0x4b, 0x6e, 0xe4, 0x63, 0x6b, 0x65, 0x62, 0x72, 0xf6, 0x64}
	if api.UTF16ToString(s) != string(utf16toutf8(s)) {
		t.Fatal("comparison fails")
	}
}

func TestMSSQLExecStoredProcedure(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop procedure dbo.temp")
	exec(t, db, `
create procedure dbo.temp
	@a	int,
	@b	int
as
begin
	return @a + @b
end
`)
	qry := `
declare @ret int
exec @ret = dbo.temp @a = ?, @b = ?
select @ret
`
	var ret int64
	if err := db.QueryRow(qry, 2, 3).Scan(&ret); err != nil {
		t.Fatal(err)
	}
	if ret != 5 {
		t.Fatalf("unexpected return value: should=5, is=%v", ret)
	}
	exec(t, db, `drop procedure dbo.temp`)
}

func TestMSSQLSingleCharParam(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, `create table dbo.temp(name nvarchar(50), age int)`)

	rows, err := db.Query("select age from dbo.temp where name=?", "v")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()

	exec(t, db, "drop table dbo.temp")
}

type tcpProxy struct {
	mu      sync.Mutex
	stopped bool
	conns   []net.Conn
}

func (p *tcpProxy) run(ln net.Listener, remote string) {
	for {
		defer p.pause()
		c1, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c1 net.Conn) {
			defer c1.Close()

			if p.paused() {
				return
			}

			p.addConn(c1)

			c2, err := net.Dial("tcp", remote)
			if err != nil {
				panic(err)
			}
			p.addConn(c2)
			defer c2.Close()

			go func() {
				io.Copy(c2, c1)
			}()
			io.Copy(c1, c2)
		}(c1)
	}
}

func (p *tcpProxy) pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	for _, c := range p.conns {
		c.Close()
	}
	p.conns = p.conns[:0]
}

func (p *tcpProxy) paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}

func (p *tcpProxy) addConn(c net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns = append(p.conns, c)
}

func (p *tcpProxy) restart() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = false
}

func TestMSSQLReconnect(t *testing.T) {
	params := newConnParams()
	address, err := params.getConnAddress()
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	err = params.updateConnAddress(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	proxy := new(tcpProxy)
	go proxy.run(ln, address)

	db, sc, err := mssqlConnectWithParams(params)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	testConn := func() error {
		var n int64
		err := db.QueryRow("select count(*) from dbo.temp").Scan(&n)
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("unexpected return value: should=1, is=%v", n)
		}
		return nil
	}

	db.Exec("drop table dbo.temp")
	exec(t, db, `create table dbo.temp (name varchar(50))`)
	exec(t, db, `insert into dbo.temp (name) values ('alex')`)

	err = testConn()
	if err != nil {
		t.Fatal(err)
	}

	proxy.pause()
	time.Sleep(100 * time.Millisecond)

	err = testConn()
	if err == nil {
		t.Fatal("database IO should fail, but succeeded")
	}

	proxy.restart()

	err = testConn()
	if err != nil {
		t.Fatal(err)
	}

	exec(t, db, "drop table dbo.temp")
}

func TestMSSQLMarkTxBadConn(t *testing.T) {
	params := newConnParams()

	address, err := params.getConnAddress()
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	err = params.updateConnAddress(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	proxy := new(tcpProxy)
	go proxy.run(ln, address)

	testFn := func(endTx func(driver.Tx) error, nextFn func(driver.Conn) error) {
		proxy.restart()

		cc, sc := drv.Stats.ConnCount, drv.Stats.StmtCount
		defer func() {
			if should, is := sc, drv.Stats.StmtCount; should != is {
				t.Errorf("leaked statement, should=%d, is=%d", should, is)
			}
			if should, is := cc, drv.Stats.ConnCount; should != is {
				t.Errorf("leaked connection, should=%d, is=%d", should, is)
			}
		}()

		dc, err := drv.Open(params.makeODBCConnectionString())
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := dc.Close(); err != nil {
				t.Fatal(err)
			}
		}()

		driverExec(nil, dc, "drop table dbo.temp")
		driverExec(t, dc, `create table dbo.temp (name varchar(50))`)

		tx, err := dc.Begin()
		if err != nil {
			t.Fatal(err)
		}

		driverExec(t, dc, `insert into dbo.temp (name) values ('alex')`)

		proxy.pause()
		time.Sleep(100 * time.Millisecond)

		// the connection is broken, ending the transaction should fail
		if err := endTx(tx); err == nil {
			t.Fatal("unexpected success, expected error")
		}

		// database/sql might return the broken driver.Conn to the pool in
		// that case the next operation must fail.
		if err := nextFn(dc); err == nil {
			t.Fatal("unexpected success, expected error")
		}
	}

	beginFn := func(dc driver.Conn) error {
		tx, err := dc.Begin()
		if err != nil {
			return err
		}
		tx.Rollback()
		return nil
	}

	prepareFn := func(dc driver.Conn) error {
		st, err := dc.Prepare(`insert into dbo.temp (name) values ('alex')`)
		if err != nil {
			return err
		}
		st.Close()
		return nil
	}

	// Test all the permutations.
	for _, endTx := range []func(driver.Tx) error{
		driver.Tx.Commit,
		driver.Tx.Rollback,
	} {
		for _, nextFn := range []func(driver.Conn) error{
			beginFn,
			prepareFn,
		} {
			testFn(endTx, nextFn)
		}
	}
}

func TestMSSQLMarkBeginBadConn(t *testing.T) {
	params := newConnParams()

	testFn := func(label string, nextFn func(driver.Conn) error) {
		cc, sc := drv.Stats.ConnCount, drv.Stats.StmtCount
		defer func() {
			if should, is := sc, drv.Stats.StmtCount; should != is {
				t.Errorf("leaked statement, should=%d, is=%d", should, is)
			}
			if should, is := cc, drv.Stats.ConnCount; should != is {
				t.Errorf("leaked connection, should=%d, is=%d", should, is)
			}
		}()

		dc, err := drv.Open(params.makeODBCConnectionString())
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := dc.Close(); err != nil {
				t.Fatal(err)
			}
		}()

		driverExec(nil, dc, "drop table dbo.temp")
		driverExec(t, dc, `create table dbo.temp (name varchar(50))`)

		// force an error starting a transaction
		func() {
			testBeginErr = errors.New("cannot start tx")
			defer func() { testBeginErr = nil }()

			if _, err := dc.Begin(); err == nil {
				t.Fatal("unexpected success, expected error")
			}
		}()

		// database/sql might return the broken driver.Conn to the pool. The
		// next operation on the driver connection must return
		// driver.ErrBadConn to prevent the bad connection from getting used
		// again.
		if should, is := driver.ErrBadConn, nextFn(dc); should != is {
			t.Errorf("%s: should=\"%v\", is=\"%v\"", label, should, is)
		}
	}

	beginFn := func(dc driver.Conn) error {
		tx, err := dc.Begin()
		if err != nil {
			return err
		}
		tx.Rollback()
		return nil
	}

	prepareFn := func(dc driver.Conn) error {
		st, err := dc.Prepare(`insert into dbo.temp (name) values ('alex')`)
		if err != nil {
			return err
		}
		st.Close()
		return nil
	}

	// Test all the permutations.
	for _, next := range []struct {
		label string
		fn    func(driver.Conn) error
	}{
		{"begin", beginFn},
		{"prepare", prepareFn},
	} {
		testFn(next.label, next.fn)
	}
}

func testMSSQLNextResultSet(t *testing.T, verifyBatch func(rows *sql.Rows)) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, `create table dbo.temp (name varchar(50))`)
	exec(t, db, `insert into dbo.temp (name) values ('russ')`)
	exec(t, db, `insert into dbo.temp (name) values ('brad')`)

	rows, err := db.Query(`
select name from dbo.temp where name = 'russ';
select name from dbo.temp where name = 'brad';
`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	verifyBatch(rows)

	exec(t, db, "drop table dbo.temp")
}

func TestMSSQLNextResultSet(t *testing.T) {
	checkName := func(rows *sql.Rows, name string) {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				t.Fatalf("executing Next for %q failed: %v", name, err)
			}
			t.Fatalf("checking %q: at least one row expected", name)
		}
		var have string
		err := rows.Scan(&have)
		if err != nil {
			t.Fatalf("executing Scan for %q failed: %v", name, err)
		}
		if name != have {
			t.Fatalf("want %q, but %q found", name, have)
		}
	}
	testMSSQLNextResultSet(t,
		func(rows *sql.Rows) {
			checkName(rows, "russ")
			if !rows.NextResultSet() {
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				t.Fatal("more result sets expected")
			}
			checkName(rows, "brad")
			if isFreeTDS() { // not sure why it does not work on FreeTDS
				t.Log("skipping broken part of the test on FreeTDS")
				return
			}
			if rows.NextResultSet() {
				t.Fatal("unexpected result set found")
			} else if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
		})
}

func TestMSSQLNextResultSetWithDifferentColumnsInResultSets(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)
	rows, err := db.Query("select 1 select 2,3")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected at least 1 result")
	}
	var v1, v2 int
	err = rows.Scan(&v1)
	if err != nil {
		t.Fatalf("unable to scan select 1 underlying error: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("expected: %v got %v", 1, v1)
	}
	if rows.Next() {
		t.Fatal("unexpected row")
	}
	if !rows.NextResultSet() {
		t.Fatal("expected another result set")
	}
	if !rows.Next() {
		t.Fatal("expected a single row")
	}
	err = rows.Scan(&v1, &v2)
	if err != nil {
		t.Fatalf("unable to scan select 2,3 underlying error: %v", err)
	}
	if v1 != 2 || v2 != 3 {
		t.Fatalf("got wrong values expected v1=%v v2=%v. got v1=%v v2=%v", 2, 3, v1, v2)
	}

}

func TestMSSQLHasNextResultSet(t *testing.T) {
	checkName := func(rows *sql.Rows, name string) {
		var reccount int
		for rows.Next() { // reading till the end of data set to trigger call into HasNextResultSet
			var have string
			err := rows.Scan(&have)
			if err != nil {
				t.Fatalf("executing Scan for %q failed: %v", name, err)
			}
			if name != have {
				t.Fatalf("want %q, but %q found", name, have)
			}
			reccount++
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("executing Next for %q failed: %v", name, err)
		}
		if reccount != 1 {
			t.Fatalf("checking %q: expected 1 row returned, but %v found", name, reccount)
		}
	}
	testMSSQLNextResultSet(t,
		func(rows *sql.Rows) {
			checkName(rows, "russ")
			if !rows.NextResultSet() {
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				t.Fatal("more result sets expected")
			}
			checkName(rows, "brad")
			if rows.NextResultSet() {
				t.Fatal("unexpected result set found")
			} else {
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
			}
		})
}

func TestMSSQLIssue127(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	db.Exec("drop table dbo.temp")
	exec(t, db, "create table dbo.temp (id int, a varchar(255))")

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err := tx.Prepare(`
DECLARE @id INT, @a VARCHAR(255)
SELECT @id = ?, @a = ?
UPDATE dbo.temp SET a = @a WHERE id = @id
IF @@ROWCOUNT = 0
  INSERT INTO dbo.temp (id, a) VALUES (@id, @a)
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stmt.Exec(1, "test"); err != nil {
		t.Errorf("Failed to insert record with ID 1: %s", err)
	}
	if _, err = stmt.Exec(1, "test2"); err != nil {
		t.Errorf("Failed to update record with ID 1: %s", err)
	}

	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
