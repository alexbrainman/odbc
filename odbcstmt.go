// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

// TODO(brainman): see if I could use SQLExecDirect anywhere

type ODBCStmt struct {
	h          api.SQLHSTMT
	Parameters []Parameter
	Cols       []Column
	// locking/lifetime
	mu         sync.Mutex
	usedByStmt bool
	usedByRows bool
}

func (c *Conn) PrepareODBCStmt(query string) (*ODBCStmt, error) {
	var out api.SQLHANDLE
	ret := api.SQLAllocHandle(api.SQL_HANDLE_STMT, api.SQLHANDLE(c.h), &out)
	if IsError(ret) {
		return nil, c.newError("SQLAllocHandle", c.h)
	}
	h := api.SQLHSTMT(out)
	drv.Stats.updateHandleCount(api.SQL_HANDLE_STMT, 1)

	b := syscall.StringByteSlice(query)
	ret = api.SQLPrepare(h, (*api.SQLCHAR)(unsafe.Pointer(&b[0])), api.SQL_NTS)
	if IsError(ret) {
		defer releaseHandle(h)
		return nil, c.newError("SQLPrepare", h)
	}
	ps, err := ExtractParameters(h)
	if err != nil {
		defer releaseHandle(h)
		return nil, err
	}
	return &ODBCStmt{
		h:          h,
		Parameters: ps,
		usedByStmt: true,
	}, nil
}

func (s *ODBCStmt) closeByStmt() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.usedByStmt {
		defer func() { s.usedByStmt = false }()
		if !s.usedByRows {
			return s.releaseHandle()
		}
	}
	return nil
}

func (s *ODBCStmt) closeByRows() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.usedByRows {
		defer func() { s.usedByRows = false }()
		if s.usedByStmt {
			ret := api.SQLCloseCursor(s.h)
			if IsError(ret) {
				return NewError("SQLCloseCursor", s.h)
			}
			return nil
		} else {
			return s.releaseHandle()
		}
	}
	return nil
}

func (s *ODBCStmt) releaseHandle() error {
	h := s.h
	s.h = api.SQLHSTMT(api.SQL_NULL_HSTMT)
	return releaseHandle(h)
}

var testingIssue5 bool // used during tests

func (s *ODBCStmt) Exec(args []driver.Value) error {
	if len(args) != len(s.Parameters) {
		return fmt.Errorf("wrong number of arguments %d, %d expected", len(args), len(s.Parameters))
	}
	for i, a := range args {
		// this could be done in 2 steps:
		// 1) bind vars right after prepare;
		// 2) set their (vars) values here;
		// but rebinding parameters for every new parameter value
		// should be efficient enough for our purpose.
		if err := s.Parameters[i].BindValue(s.h, i, a); err != nil {
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

func (s *ODBCStmt) BindColumns() error {
	// count columns
	var n api.SQLSMALLINT
	ret := api.SQLNumResultCols(s.h, &n)
	if IsError(ret) {
		return NewError("SQLNumResultCols", s.h)
	}
	if n < 1 {
		return errors.New("Stmt did not create a result set")
	}
	// fetch column descriptions
	s.Cols = make([]Column, n)
	binding := true
	for i := range s.Cols {
		c, err := NewColumn(s.h, i)
		if err != nil {
			return err
		}
		s.Cols[i] = c
		// Once we found one non-bindable column, we will not bind the rest.
		// http://www.easysoft.com/developer/languages/c/odbc-tutorial-fetching-results.html
		// ... One common restriction is that SQLGetData may only be called on columns after the last bound column. ...
		if !binding {
			continue
		}
		bound, err := s.Cols[i].Bind(s.h, i)
		if err != nil {
			return err
		}
		if !bound {
			binding = false
		}
	}
	return nil
}
