// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"context"
	"database/sql/driver"
	"errors"

	"github.com/alexbrainman/odbc/api"
)

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
}

// implement driver.Stmt
func (s *Stmt) NumInput() int {
	if s.parameters == nil {
		return -1
	}
	return len(s.parameters)
}

// implement driver.Stmt
// Close closes the statement.
//
// As of Go 1.1, a Stmt will not be closed if it's in use
// by any queries.
func (s *Stmt) Close() error {
	if s.c.closingInBG.Load() {
		//if we are cancelling/closing in a background thread, ignore requests to Close this statement from the driver
		return nil
	}
	return s.close()
}
func (s *Stmt) close() error {
	return s.releaseHandle()
}

// implement driver.Stmt - per documentation, not supposed to be used by multiple goroutines
func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), toNamedValues(args))
}

// implement driver.Stmt - per documentation, not supposed to be used by multiple goroutines
func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), toNamedValues(args))
}

func (s *Stmt) releaseHandle() error {
	h := s.h
	s.h = api.SQLHSTMT(api.SQL_NULL_HSTMT)
	return releaseHandle(h)
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

func toNamedValues(values []driver.Value) []driver.NamedValue {
	namedValues := make([]driver.NamedValue, len(values))
	for idx, value := range values {
		namedValues[idx] = driver.NamedValue{
			Name:    "",
			Ordinal: idx + 1,
			Value:   value,
		}
	}
	return namedValues
}
