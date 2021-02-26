package odbc

import (
	"testing"
)

func TestMSSQLMultipleExecOnStatement(t *testing.T) {

	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	exec(t, db, "drop table if exists dbo.temp")
	exec(t, db, "create table dbo.temp (id int, a varchar(255))")

	stmt1, err := db.Prepare(`insert into dbo.temp (id, a) VALUES (?,?)`)

	if err != nil {
		t.Fatal(err)
	}
	if _, err = stmt1.Exec(1, "TEST 1"); err != nil {
		t.Errorf("Failed to insert record with ID 1: %s", err)
	}
	if _, err = stmt1.Exec(2, "TEST 2"); err != nil {
		t.Errorf("Failed to insert record with ID 2: %s", err)
	}

	if err = stmt1.Close(); err != nil {
		t.Errorf("Failed to close exec statement: %s", err)
	}

	stmt2, err := db.Prepare(`SELECT a FROM dbo.temp WHERE id = ?`)

	if err != nil {
		t.Fatal(err)
	}
	if rows, err := stmt2.Query(1); err != nil {
		t.Fatalf("Failed to query record with ID 1: %s", err)
	} else if found := rows.Next(); !found {
		t.Fatalf("No results returned from query")
	} else {
		var field string
		rows.Scan(&field)
		if field != "TEST 1" {
			t.Fatalf("Got unexpected value from query: %s", field)
		}
		rows.Close()
	}

	if rows, err := stmt2.Query(2); err != nil {
		t.Fatalf("Failed to query record with ID 2: %s", err)
	} else if found := rows.Next(); !found {
		t.Fatalf("No results returned from query")
	} else {
		var field string
		rows.Scan(&field)
		if field != "TEST 2" {
			t.Fatalf("Got unexpected value from query: %s", field)
		}
		rows.Close()
	}

	if err = stmt2.Close(); err != nil {
		t.Errorf("Failed to close query statement: %s", err)
	}
}

//
//func TestMSSQLQueryContextTimeout(t *testing.T) {
//	db, sc, err := mssqlConnect()
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer closeDB(t, db, sc, sc)
//
//	testingIssue5 = false
//
//	start := time.Now()
//	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
//	defer cancel()
//
//	if _, qErr := db.QueryContext(ctx, "WAITFOR DELAY '00:00:10'"); qErr == nil {
//		t.Fatal("expected an error to be returned")
//	} else if !errors.Is(qErr, context.DeadlineExceeded) {
//		t.Fatalf("expected a context canceled error. got: %s", qErr.Error())
//	}
//	if time.Since(start).Seconds() > 2 {
//		t.Fatal("query should have been canceled after 1 second")
//	}
//
//	if _, qErr := db.QueryContext(ctx, "SELECT 1"); qErr == nil {
//		t.Fatal("expected an error to be returned for subsequent query on expired context")
//	} else if !errors.Is(qErr, context.DeadlineExceeded) {
//		t.Fatalf("expected a context canceled error. got: %s", qErr.Error())
//	}
//
//	if rows, qErr := db.QueryContext(context.Background(), "SELECT 1"); qErr != nil {
//		t.Fatalf("query on a fresh context should execute without error.  Got: %s", qErr.Error())
//	} else {
//		rows.Close()
//	}
//}
//
//func TestMSSQLExecContextTimeout(t *testing.T) {
//	db, sc, err := mssqlConnect()
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer closeDB(t, db, sc, sc)
//
//	testingIssue5 = false
//
//	start := time.Now()
//	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
//	defer cancel()
//
//	if _, qErr := db.ExecContext(ctx, "WAITFOR DELAY '00:00:10'"); qErr == nil {
//		t.Fatal("expected an error to be returned")
//	} else if !errors.Is(qErr, context.DeadlineExceeded) {
//		t.Fatalf("expected a context canceled error. got: %s", qErr.Error())
//	}
//	if time.Since(start).Seconds() > 2 {
//		t.Fatal("exec should have been canceled after 1 second")
//	}
//
//	if _, qErr := db.ExecContext(ctx, "SELECT 1"); qErr == nil {
//		t.Fatal("expected an error to be returned for subsequent exec on expired context")
//	} else if !errors.Is(qErr, context.DeadlineExceeded) {
//		t.Fatalf("expected a context canceled error. got: %s", qErr.Error())
//	}
//
//	if _, qErr := db.ExecContext(context.Background(), "SELECT 1"); qErr != nil {
//		t.Fatalf("exec on a fresh context should execute without error.  Got: %s", qErr.Error())
//	}
//}
