// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"code.google.com/p/odbc/api"
	"database/sql/driver"
	"errors"
	"sync"
)

type Stmt struct {
	c     *Conn
	query string
	os    *ODBCStmt
	mu    sync.Mutex
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	os, err := c.PrepareODBCStmt(query)
	if err != nil {
		return nil, err
	}
	return &Stmt{c: c, os: os, query: query}, nil
}

func (s *Stmt) NumInput() int {
	if s.os == nil {
		return -1
	}
	return len(s.os.Parameters)
}

func (s *Stmt) Close() error {
	if s.os == nil {
		return errors.New("Stmt is already closed")
	}
	ret := s.os.closeByStmt()
	s.os = nil
	return ret
}

func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	if s.os == nil {
		return nil, errors.New("Stmt is closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.os.usedByRows {
		s.os.closeByStmt()
		s.os = nil
		os, err := s.c.PrepareODBCStmt(s.query)
		if err != nil {
			return nil, err
		}
		s.os = os
	}
	err := s.os.Exec(args)
	if err != nil {
		return nil, err
	}
	var c api.SQLLEN
	ret := api.SQLRowCount(s.os.h, &c)
	if IsError(ret) {
		return nil, NewError("SQLRowCount", s.os.h)
	}
	return &Result{rowCount: int64(c)}, nil
}

func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.os == nil {
		return nil, errors.New("Stmt is closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.os.usedByRows {
		s.os.closeByStmt()
		s.os = nil
		os, err := s.c.PrepareODBCStmt(s.query)
		if err != nil {
			return nil, err
		}
		s.os = os
	}
	err := s.os.Exec(args)
	if err != nil {
		return nil, err
	}
	err = s.os.BindColumns()
	if err != nil {
		return nil, err
	}
	s.os.usedByRows = true // now both Stmt and Rows refer to it
	return &Rows{os: s.os}, nil
}
