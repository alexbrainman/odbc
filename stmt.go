// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/alexbrainman/odbc/api"
)

var ErrStmtClosed = errors.New("statement is closed")

// TODO(brainman): see if I could use SQLExecDirect anywhere

type Stmt struct {
	c     *Conn
	query string

	h          api.SQLHSTMT
	parameters []Parameter
	cols       []Column

	//each statement can only have one open rows.  If a second query is executed while rows is still open,
	//the driver will prepare a new statement to execute on
	rows *Rows

	closed bool
	ctx    context.Context
}

// implement driver.Stmt
func (s *Stmt) NumInput() int {
	if s.closed {
		return -1
	}
	return len(s.parameters)
}

// implement driver.Stmt
func (s *Stmt) Close() error {
	if s.closed {
		return ErrStmtClosed
	}
	s.closed = true

	if s.rows == nil {
		return s.releaseHandle()
	}

	return nil
}

// implement driver.Stmt - per documentation, not supposed to be used by multiple goroutines
func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	if s.closed {
		return nil, ErrStmtClosed
	}

	if err := s.exec(args, s.c); err != nil {
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

// implement driver.Stmt - per documentation, not supposed to be used by multiple goroutines
func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.closed {
		return nil, ErrStmtClosed
	}

	if err := s.exec(args, s.c); err != nil {
		return nil, err
	}

	if err := s.bindColumns(); err != nil {
		return nil, err
	}

	s.rows = &Rows{s: s}
	return s.rows, nil
}

func (s *Stmt) releaseHandle() error {
	h := s.h
	s.h = api.SQLHSTMT(api.SQL_NULL_HSTMT)
	return releaseHandle(h)
}

var testingIssue5 bool // used during tests

func (s *Stmt) exec(args []driver.Value, conn *Conn) error {
	if len(args) != len(s.parameters) {
		return fmt.Errorf("wrong number of arguments %d, %d expected", len(args), len(s.parameters))
	}
	for i, a := range args {
		// this could be done in 2 steps:
		// 1) bind vars right after prepare;
		// 2) set their (vars) values here;
		// but rebinding parameters for every new parameter value
		// should be efficient enough for our purpose.
		if err := s.parameters[i].BindValue(s.h, i, a, conn); err != nil {
			return err
		}
	}
	if testingIssue5 {
		time.Sleep(10 * time.Microsecond)
	}
	ret := api.SQLExecute(s.h)
	if ret == api.SQL_NO_DATA {
		// success but no data to report
		return nil
	}
	if IsError(ret) {
		return NewError("SQLExecute", s.h)
	}
	return nil
}

func (s *Stmt) bindColumns() error {
	// count columns
	var n api.SQLSMALLINT
	ret := api.SQLNumResultCols(s.h, &n)
	if IsError(ret) {
		return NewError("SQLNumResultCols", s.h)
	}
	if n < 1 {
		return errors.New("statement did not create a result set")
	}
	// fetch column descriptions
	s.cols = make([]Column, n)
	binding := true
	for i := range s.cols {
		c, err := NewColumn(s.h, i)
		if err != nil {
			return err
		}
		s.cols[i] = c
		// Once we found one non-bindable column, we will not bind the rest.
		// http://www.easysoft.com/developer/languages/c/odbc-tutorial-fetching-results.html
		// ... One common restriction is that SQLGetData may only be called on columns after the last bound column. ...
		if !binding {
			continue
		}
		bound, err := s.cols[i].Bind(s.h, i)
		if err != nil {
			return err
		}
		if !bound {
			binding = false
		}
	}
	return nil
}
