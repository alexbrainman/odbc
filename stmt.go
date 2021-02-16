// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/alexbrainman/odbc/api"
)

type Stmt struct {
	c     *Conn
	query string

	h          api.SQLHSTMT
	parameters []Parameter
	cols       []Column
	// locking/lifetime
	mu         sync.Mutex
	usedByStmt bool
	usedByRows bool

	closed *atomic.Value
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	if c.bad.Load().(bool) {
		return nil, driver.ErrBadConn
	}
	return c.prepareODBCStmt(query)
}

func (s *Stmt) NumInput() int {
	if s.closed.Load().(bool) {
		return -1
	}
	return len(s.parameters)
}

func (s *Stmt) Close() error {
	if s.closed.Load().(bool) {
		return errors.New("Stmt is already closed")
	}
	ret := s.closeByStmt()
	s.closed.Store(true)
	return ret
}

func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	if s.closed.Load().(bool) {
		return nil, errors.New("Stmt is closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.usedByRows {
		s.closeByStmt()
		s.closed.Store(true)
		os, err := s.c.prepareODBCStmt(s.query)
		if err != nil {
			return nil, err
		}
		*s = *os
	}
	err := s.exec(args, s.c)
	if err != nil {
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

func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.closed.Load().(bool) {
		return nil, errors.New("Stmt is closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.usedByRows {
		s.closeByStmt()
		s.closed.Store(true)
		os, err := s.c.prepareODBCStmt(s.query)
		if err != nil {
			return nil, err
		}
		*s = *os
	}
	err := s.exec(args, s.c)
	if err != nil {
		return nil, err
	}
	err = s.bindColumns()
	if err != nil {
		return nil, err
	}
	s.usedByRows = true // now both Stmt and Rows refer to it
	return &Rows{s: s}, nil
}
