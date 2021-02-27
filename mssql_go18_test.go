package odbc

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
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

func TestMSSQLContextExpired(t *testing.T) {

	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	expiredContext, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	if _, err := db.PrepareContext(expiredContext, `insert into dbo.temp (id, a) VALUES (?,?)`); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected a context expired error from PrepareContext")
	}

	if _, err := db.QueryContext(expiredContext, `SELECT * FROM dbo.temp`); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected a context expired error from QueryContext")
	}

	if _, err := db.ExecContext(expiredContext, `SELECT 1`); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected a context expired error from ExecContext")
	}

	if _, err := db.BeginTx(expiredContext, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected a context expired error from BeginTx")
	}
}

func TestMSSQLQueryContextTimeout(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}

	testingIssue5 = false

	start := time.Now()

	var wgTest sync.WaitGroup

	for i := 0; i < 10; i++ {
		wgTest.Add(1)
		go func() {
			defer wgTest.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if _, qErr := db.QueryContext(ctx, "WAITFOR DELAY '00:00:10'"); qErr == nil {
				t.Error("expected an error to be returned")
			} else if !errors.Is(qErr, context.DeadlineExceeded) {
				t.Errorf("expected a context deadline error. got: %s", qErr.Error())
			}
			if time.Since(start).Seconds() >= 2 {
				t.Error("query should have been canceled after 1 second")
			}
		}()
	}

	wgTest.Wait()

	//wait for the query to finish cancelling in the background and for the connection to close
	time.Sleep(3 * time.Second)

	closeDB(t, db, sc, sc)

}

func TestMSSQLExecContextTimeout(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}

	testingIssue5 = false

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if _, qErr := db.ExecContext(ctx, "WAITFOR DELAY '00:00:10'"); qErr == nil {
		t.Fatal("expected an error to be returned")
	} else if !errors.Is(qErr, context.DeadlineExceeded) {
		t.Fatalf("expected a context canceled error. got: %s", qErr.Error())
	}
	if time.Since(start).Seconds() > 2 {
		t.Fatal("exec should have been canceled after 1 second")
	}

	if _, qErr := db.ExecContext(ctx, "SELECT 1"); qErr == nil {
		t.Fatal("expected an error to be returned for subsequent exec on expired context")
	} else if !errors.Is(qErr, context.DeadlineExceeded) {
		t.Fatalf("expected a context canceled error. got: %s", qErr.Error())
	}

	if _, qErr := db.ExecContext(context.Background(), "SELECT 1"); qErr != nil {
		t.Fatalf("exec on a fresh context should execute without error.  Got: %s", qErr.Error())
	}

	//wait for the query to finish cancelling in the background and for the connection to close
	time.Sleep(3 * time.Second)

	closeDB(t, db, sc, sc)
}

func TestMSSQLPing(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	c, _ := db.Conn(context.Background())

	if err := c.PingContext(context.Background()); err != nil {
		t.Fatalf("did not expect an error from ping. got %s", err.Error())
	}

	c.Close()

	if err := c.PingContext(context.Background()); err == nil {
		t.Fatalf("expected ping to fail after being closed")
	}

}

func TestMSSQLTxOptions(t *testing.T) {
	db, sc, err := mssqlConnect()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB(t, db, sc, sc)

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
		ReadOnly:  true,
	})
	if err != nil {
		t.Fatalf("expected no error starting transaction.  Got %s", err.Error())
	}
	if _, err = tx.ExecContext(context.Background(), "SELECT 1"); err != nil {
		t.Fatalf("expected no error from exec on transaction.  Got %s", err.Error())
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("expected no error rolling back transaction.  Got %s", err.Error())
	}

}
