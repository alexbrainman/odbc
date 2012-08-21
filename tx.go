// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"code.google.com/p/odbc/api"
	"database/sql/driver"
	"errors"
)

type Tx struct {
	c *Conn
}

func (c *Conn) setAutoCommitAttr(a uintptr) error {
	ret := api.SQLSetConnectAttr(c.h, api.SQL_ATTR_AUTOCOMMIT,
		api.SQLPOINTER(a), api.SQL_IS_UINTEGER)
	if IsError(ret) {
		return NewError("SQLSetConnectAttr", c.h)
	}
	return nil
}

func (c *Conn) Begin() (driver.Tx, error) {
	if c.tx != nil {
		return nil, errors.New("already in a transaction")
	}
	c.tx = &Tx{c: c}
	err := c.setAutoCommitAttr(api.SQL_AUTOCOMMIT_OFF)
	if err != nil {
		return nil, err
	}
	return c.tx, nil
}

func (c *Conn) endTx(commit bool) error {
	if c.tx == nil {
		return errors.New("not in a transaction")
	}
	c.tx = nil
	var howToEnd api.SQLSMALLINT
	if commit {
		howToEnd = api.SQL_COMMIT
	} else {
		howToEnd = api.SQL_ROLLBACK
	}
	ret := api.SQLEndTran(api.SQL_HANDLE_DBC, api.SQLHANDLE(c.h), howToEnd)
	if IsError(ret) {
		return NewError("SQLEndTran", c.h)
	}
	err := c.setAutoCommitAttr(api.SQL_AUTOCOMMIT_ON)
	if err != nil {
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
