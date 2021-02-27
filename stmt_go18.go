package odbc

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/alexbrainman/odbc/api"
)

//implement driver.StmtExecContext
func (s *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {

	if err := s.exec(ctx, args); err != nil {
		return nil, err
	}

	var sumRowCount int64
	for {
		var c api.SQLLEN
		ret := api.SQLRowCount(s.h, &c)
		if IsError(ret) {
			return nil, NewError("SQLRowCount", s.h)
		}
		sumRowCount += int64(c)
		if ret = api.SQLMoreResults(s.h); ret == api.SQL_NO_DATA {
			break
		}
	}
	return &Result{rowCount: sumRowCount}, nil
}

//implement driver.StmtQueryContext
func (s *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := s.exec(ctx, args); err != nil {
		return nil, err
	}

	if err := s.bindColumns(); err != nil {
		return nil, err
	}

	s.rows = &Rows{s: s}
	return s.rows, nil

}

var testingIssue5 bool // used during tests

func (s *Stmt) exec(ctx context.Context, args []driver.NamedValue) error {
	if len(args) != len(s.parameters) {
		return fmt.Errorf("wrong number of arguments %d, %d expected", len(args), len(s.parameters))
	}
	for _, namedValue := range args {
		// this could be done in 2 steps:
		// 1) bind vars right after prepare;
		// 2) set their (vars) values here;
		// but rebinding parameters for every new parameter value
		// should be efficient enough for our purpose.
		if err := s.parameters[namedValue.Ordinal-1].BindValue(s.h, namedValue.Ordinal-1, namedValue.Value, s.c); err != nil {
			return err
		}
	}
	if testingIssue5 {
		time.Sleep(10 * time.Microsecond)
	}

	sqlResult, cancelExec := s.sqlExecuteAsync()

	select {
	case <-ctx.Done():
		//mark the connection as bad, so that the driver does not reuse it
		s.c.bad.Store(true)
		//mark the statement as closing in bg so stmt.Close and conn.Close do not block
		s.c.closingInBG.Store(true)
		//cancel the query and close the statement and connection in the background
		go cancelExec()
		return ctx.Err()
	case err := <-sqlResult:
		return err
	}

}

func (s *Stmt) sqlExecuteAsync() (err <-chan error, cancel func()) {
	var wgExecuting sync.WaitGroup
	cancelled := atomic.NewBool(false)
	cancel = func() {
		if cancelled.Load() {
			return
		}
		cancelled.Store(true)
		//cancel the running statement
		_ = api.SQLCancel(s.h)
		//wait for the query to finish
		wgExecuting.Wait()
		s.close()
		s.c.close()
	}
	errChannel := make(chan error)
	wgExecuting.Add(1)
	go func() {
		defer wgExecuting.Done()
		ret := api.SQLExecute(s.h)
		if !cancelled.Load() {
			var execErr error
			if ret != api.SQL_NO_DATA && IsError(ret) {
				execErr = NewError("SQLExecute", s.h)
			}
			errChannel <- execErr
		}
	}()
	return errChannel, cancel
}
