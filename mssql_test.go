// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql"
	"flag"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var (
	mssrv    = flag.String("mssrv", "server", "ms sql server name")
	msdb     = flag.String("msdb", "dbname", "ms sql server database name")
	msuser   = flag.String("msuser", "", "ms sql server user name")
	mspass   = flag.String("mspass", "", "ms sql server password")
	msdriver = flag.String("msdriver", "sql server", "ms sql odbc driver name")
)

func mssqlConnect() (db *sql.DB, stmtCount int, err error) {
	var params map[string]string
	if runtime.GOOS == "windows" {
		params = map[string]string{
			"driver":   *msdriver,
			"server":   *mssrv,
			"database": *msdb,
		}
		if len(*msuser) == 0 {
			params["trusted_connection"] = "yes"
		} else {
			params["uid"] = *msuser
			params["pwd"] = *mspass
		}
	} else {
		params = map[string]string{
			"driver":   "freetds",
			"server":   *mssrv,
			"port":     "1433",
			"database": *msdb,
			"uid":      *msuser,
			"pwd":      *mspass,
			//"clientcharset": "UTF-8",
			//"debugflags": "0xffff",
		}
	}
	var c string
	for n, v := range params {
		c += n + "=" + v + ";"
	}
	db, err = sql.Open("odbc", c)
	if err != nil {
		return nil, 0, err
	}
	return db, db.Driver().(*Driver).Stats.StmtCount, nil
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

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func exec(t *testing.T, db *sql.DB, query string) {
	// TODO(brainman): make sure http://code.google.com/p/go/issues/detail?id=3678 is fixed
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
			dob:       time.Date(2009, 5, 10, 11, 1, 1, 0, time.Local),
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
			t.Errorf("I did not know, that %s weights %dkg (%dkg expected)", name, is.weight, want.weight)
			continue
		}
		if !is.dob.Equal(want.dob) {
			t.Errorf("I did not know, that %s's date of birth is %v (%v expected)", name, is.dob, want.dob)
			continue
		}
		if !equal(is.data, want.data) {
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
			expect := a.(bool)
			if got != expect {
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case int32:
			expect := a.(int32)
			if got != expect {
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case int64:
			expect := a.(int64)
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
			expect := a.(string)
			if got != expect {
				return fmt.Errorf("expect %q, but got %q", expect, got)
			}
		case []byte:
			expect := a.([]byte)
			if !equal(got, expect) {
				return fmt.Errorf("expect %v, but got %v", expect, got)
			}
		case time.Time:
			expect := a.(time.Time)
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
	{"select cast(123 as varchar(21))", match("123")},
	{"select cast(123 as char(5))", match("123  ")},
	{"select cast('abcde' as varchar(3))", match("abc")},
	{"select cast('' as varchar(5))", match("")},
	{"select cast(NULL as varchar(5))", match(nil)},
	{"select cast(123 as nvarchar(21))", match("123")},
	{"select cast('abcde' as nvarchar(3))", match("abc")},
	{"select cast('' as nvarchar(5))", match("")},
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
	{"select cast('abc' as varchar(max))", match("abc")},
	{fmt.Sprintf("select cast('%s' as varchar(max))", veryLongString), match(veryLongString)},
	{"select cast(NULL as varchar(max))", match(nil)},
	{"select cast('abc' as nvarchar(max))", match("abc")},
	{fmt.Sprintf("select cast('%s' as nvarchar(max))", veryLongString), match(veryLongString)},
	{"select cast(NULL as nvarchar(max))", match(nil)},
	{"select cast('abc' as text)", match("abc")},
	{fmt.Sprintf("select cast('%s' as text)", veryLongString), match(veryLongString)},
	{"select cast(NULL as text)", match(nil)},
	{"select cast('abc' as ntext)", match("abc")},
	{fmt.Sprintf("select cast('%s' as ntext)", veryLongString), match(veryLongString)},
	{"select cast(NULL as ntext)", match(nil)},

	// binary blobs
	{"select cast('abc' as binary(5))", match([]byte{'a', 'b', 'c', 0, 0})},
	{"select cast(NULL as binary(5))", match(nil)},
	{"select cast('abc' as varbinary(5))", match([]byte{'a', 'b', 'c'})},
	{"select cast(NULL as varbinary(5))", match(nil)},
	{"select cast('abc' as varbinary(max))", match([]byte{'a', 'b', 'c'})},
	{fmt.Sprintf("select cast('%s' as varbinary(max))", veryLongString), match([]byte(veryLongString))},
	{"select cast(NULL as varbinary(max))", match(nil)},
}

// TODO(brainman): see why typeWindowsSpecificTests do not work on linux

var typeWindowsSpecificTests = []typeTest{
	{"select cast(N'\u0421\u0430\u0448\u0430' as nvarchar(5))", match("\u0421\u0430\u0448\u0430")},
	{"select cast(N'\u0421\u0430\u0448\u0430' as nvarchar(max))", match("\u0421\u0430\u0448\u0430")},
	{"select cast(N'\u0421\u0430\u0448\u0430' as ntext)", match("\u0421\u0430\u0448\u0430")},
	{"select cast(N'<root>hello</root>' as xml)", match("<root>hello</root>")},
	{"select cast(N'<root><doc><item1>dd</item1></doc></root>' as xml)", match("<root><doc><item1>dd</item1></doc></root>")},
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
	if runtime.GOOS == "windows" {
		tests = append(tests, typeWindowsSpecificTests...)
	}
	for _, r := range tests {
		rows, err := db.Query(r.query)
		if err != nil {
			t.Errorf("db.Query(%q) failed: %v", r.query, err)
			continue
		}
		defer rows.Close()
		for rows.Next() {
			var got interface{}
			err := rows.Scan(&got)
			if err != nil {
				t.Errorf("rows.Scan for %q failed: %v", r.query, err)
				continue
			}
			err = r.match(got)
			if err != nil {
				t.Errorf("test %q failed: %v", r.query, err)
			}
		}
		err = rows.Err()
		if err != nil {
			t.Error(err)
			continue
		}
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
		// http://code.google.com/p/go/issues/detail?id=3865
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

	// no reource tracking past this point

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

	const nworkers = 8
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
