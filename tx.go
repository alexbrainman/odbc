// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"errors"

	"github.com/alexbrainman/odbc/api"
)

type Tx struct {
	c *Conn
}

var testBeginErr error // used during tests

func (c *Conn) setAutoCommitAttr(a uintptr) error {
	if testBeginErr != nil {
		return testBeginErr
	}
	ret := api.SQLSetConnectUIntPtrAttr(c.h, api.SQL_ATTR_AUTOCOMMIT, a, api.SQL_IS_UINTEGER)
	if IsError(ret) {
		return c.newError("SQLSetConnectUIntPtrAttr", c.h)
	}
	return nil
}

func (c *Conn) Begin() (driver.Tx, error) {
	if c.bad {
		return nil, driver.ErrBadConn
	}
	if c.tx != nil {
		return nil, errors.New("already in a transaction")
	}
	c.tx = &Tx{c: c}
	err := c.setAutoCommitAttr(api.SQL_AUTOCOMMIT_OFF)
	if err != nil {
		c.bad = true
		return nil, err
	}
	return c.tx, nil
}

func (c *Conn) endTx(commit bool) error {
	if c.tx == nil {
		return errors.New("not in a transaction")
	}
	var howToEnd api.SQLSMALLINT
	if commit {
		howToEnd = api.SQL_COMMIT
	} else {
		howToEnd = api.SQL_ROLLBACK
	}
	ret := api.SQLEndTran(api.SQL_HANDLE_DBC, api.SQLHANDLE(c.h), howToEnd)
	if IsError(ret) {
		c.bad = true
		return c.newError("SQLEndTran", c.h)
	}
	c.tx = nil
	err := c.setAutoCommitAttr(api.SQL_AUTOCOMMIT_ON)
	if err != nil {
		c.bad = true
		return err
	}
	return nil
}

func (tx *Tx) Commit() error {
	return tx.c.endTx(true)
}

func (tx *Tx) Rollback() error {
	return tx.c.endTx(false)
}
