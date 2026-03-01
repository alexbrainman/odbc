// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
)

var (
	ifxDSN = flag.String("ifxdsn", "", "Informix DSN for testing (e.g. DSN=Informix;UID=rx;PWD=pass)")
)

func ifxConnect(t *testing.T) *sql.DB {
	t.Helper()
	dsn := *ifxDSN
	if dsn == "" {
		dsn = os.Getenv("INFORMIX_TEST_DSN")
	}
	if dsn == "" {
		t.Skip("skipping Informix test: set -ifxdsn flag or INFORMIX_TEST_DSN env")
	}
	db, err := sql.Open("odbc", dsn)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// TestInformixPing verifies that driver.Pinger works via
// SQL_ATTR_CONNECTION_DEAD.
func TestInformixPing(t *testing.T) {
	db := ifxConnect(t)
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

// TestInformixINT8 verifies that Informix INT8 (vendor type -114)
// is read as int64 without needing a CAST.
func TestInformixINT8(t *testing.T) {
	db := ifxConnect(t)
	defer db.Close()

	// Create temp table with INT8 column.
	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_int8_odbc;
		CREATE TEMP TABLE test_int8_odbc (id SERIAL PRIMARY KEY, big_val INT8);
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Insert a value that exceeds int32 range.
	var want int64 = 9000000001
	_, err = db.Exec("INSERT INTO test_int8_odbc (big_val) VALUES (?)", want)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got int64
	err = db.QueryRow("SELECT big_val FROM test_int8_odbc WHERE big_val = ?", want).Scan(&got)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != want {
		t.Errorf("INT8: got %d, want %d", got, want)
	}
}

// TestInformixBOOLEAN verifies that Informix BOOLEAN (vendor type 41)
// is read as bool without needing a CASE expression.
func TestInformixBOOLEAN(t *testing.T) {
	db := ifxConnect(t)
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_bool_odbc;
		CREATE TEMP TABLE test_bool_odbc (id SERIAL PRIMARY KEY, flag BOOLEAN);
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO test_bool_odbc (flag) VALUES (?)", true)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got bool
	err = db.QueryRow("SELECT flag FROM test_bool_odbc WHERE id = 1").Scan(&got)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if !got {
		t.Error("BOOLEAN: expected true, got false")
	}
}

// TestInformixDATETIME verifies that Informix DATETIME YEAR TO SECOND
// (vendor type -100) is read as time.Time without needing a CAST.
func TestInformixDATETIME(t *testing.T) {
	db := ifxConnect(t)
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_datetime_odbc;
		CREATE TEMP TABLE test_datetime_odbc (
			id SERIAL PRIMARY KEY,
			ts DATETIME YEAR TO SECOND
		);
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	want := time.Date(2026, 2, 20, 8, 30, 45, 0, time.Local)
	_, err = db.Exec("INSERT INTO test_datetime_odbc (ts) VALUES (?)", want)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got time.Time
	err = db.QueryRow("SELECT ts FROM test_datetime_odbc WHERE id = 1").Scan(&got)
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	// Compare without nanoseconds (Informix DATETIME YEAR TO SECOND
	// has no fractional seconds).
	gotTrunc := got.Truncate(time.Second)
	wantTrunc := want.Truncate(time.Second)
	if !gotTrunc.Equal(wantTrunc) {
		t.Errorf("DATETIME: got %v, want %v", gotTrunc, wantTrunc)
	}
}

// TestInformixMixedTypes verifies a query with INT8, BOOLEAN, DATETIME,
// VARCHAR, and INTEGER columns all in one result set.
func TestInformixMixedTypes(t *testing.T) {
	db := ifxConnect(t)
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_mixed_odbc;
		CREATE TEMP TABLE test_mixed_odbc (
			id          SERIAL PRIMARY KEY,
			big_val     INT8,
			flag        BOOLEAN,
			ts          DATETIME YEAR TO SECOND,
			name        VARCHAR(50),
			small_val   INTEGER
		);
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	ts := time.Date(2026, 1, 15, 14, 30, 0, 0, time.Local)
	_, err = db.Exec(
		"INSERT INTO test_mixed_odbc (big_val, flag, ts, name, small_val) VALUES (?, ?, ?, ?, ?)",
		int64(9000000001), true, ts, "pharmacy-test", int64(42),
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var (
		bigVal   int64
		flag     bool
		gotTS    time.Time
		name     string
		smallVal int32
	)
	err = db.QueryRow("SELECT big_val, flag, ts, name, small_val FROM test_mixed_odbc WHERE id = 1").Scan(
		&bigVal, &flag, &gotTS, &name, &smallVal,
	)
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	var errs []string
	if bigVal != 9000000001 {
		errs = append(errs, fmt.Sprintf("INT8: got %d, want 9000000001", bigVal))
	}
	if !flag {
		errs = append(errs, "BOOLEAN: got false, want true")
	}
	if !gotTS.Truncate(time.Second).Equal(ts.Truncate(time.Second)) {
		errs = append(errs, fmt.Sprintf("DATETIME: got %v, want %v", gotTS, ts))
	}
	if name != "pharmacy-test" {
		errs = append(errs, fmt.Sprintf("VARCHAR: got %q, want %q", name, "pharmacy-test"))
	}
	if smallVal != 42 {
		errs = append(errs, fmt.Sprintf("INTEGER: got %d, want 42", smallVal))
	}
	for _, e := range errs {
		t.Error(e)
	}
}

// TestInformixNullINT8 verifies NULL handling for INT8 columns.
func TestInformixNullINT8(t *testing.T) {
	db := ifxConnect(t)
	defer db.Close()

	_, err := db.Exec(`
		DROP TABLE IF EXISTS test_null_int8_odbc;
		CREATE TEMP TABLE test_null_int8_odbc (id SERIAL PRIMARY KEY, big_val INT8);
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO test_null_int8_odbc (big_val) VALUES (NULL)")
	if err != nil {
		t.Fatalf("insert null: %v", err)
	}

	var got sql.NullInt64
	err = db.QueryRow("SELECT big_val FROM test_null_int8_odbc WHERE id = 1").Scan(&got)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if got.Valid {
		t.Errorf("expected NULL INT8, got %d", got.Int64)
	}
}
