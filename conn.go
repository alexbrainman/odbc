// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"errors"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

type Conn struct {
	h                api.SQLHDBC
	tx               *Tx
	bad              *atomic.Value
	isMSAccessDriver bool
}

var accessDriverSubstr = strings.ToUpper(strings.Replace("DRIVER={Microsoft Access Driver", " ", "", -1))

func (c *Conn) Close() (err error) {
	if c.tx != nil {
		c.tx.Rollback()
	}
	h := c.h
	defer func() {
		c.h = api.SQLHDBC(api.SQL_NULL_HDBC)
		e := releaseHandle(h)
		if err == nil {
			err = e
		}
	}()
	ret := api.SQLDisconnect(c.h)
	if IsError(ret) {
		return c.newError("SQLDisconnect", h)
	}
	return err
}

func (c *Conn) newError(apiName string, handle interface{}) error {
	err := NewError(apiName, handle)
	if err == driver.ErrBadConn {
		c.bad.Store(true)
	}
	return err
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	if c.bad.Load().(bool) {
		return nil, driver.ErrBadConn
	}

	var out api.SQLHANDLE
	ret := api.SQLAllocHandle(api.SQL_HANDLE_STMT, api.SQLHANDLE(c.h), &out)
	if IsError(ret) {
		return nil, c.newError("SQLAllocHandle", c.h)
	}
	h := api.SQLHSTMT(out)
	err := drv.Stats.updateHandleCount(api.SQL_HANDLE_STMT, 1)
	if err != nil {
		return nil, err
	}

	b := api.StringToUTF16(query)
	ret = api.SQLPrepare(h, (*api.SQLWCHAR)(unsafe.Pointer(&b[0])), api.SQL_NTS)
	if IsError(ret) {
		defer releaseHandle(h)
		return nil, c.newError("SQLPrepare", h)
	}
	ps, err := ExtractParameters(h)
	if err != nil {
		defer releaseHandle(h)
		return nil, err
	}
	closed := &atomic.Value{}
	closed.Store(false)
	return &Stmt{
		c:          c,
		query:      query,
		h:          h,
		parameters: ps,
		usedByStmt: true,
		closed:     closed,
	}, nil
}


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
	if c.bad.Load().(bool) {
		return nil, driver.ErrBadConn
	}
	if c.tx != nil {
		return nil, errors.New("already in a transaction")
	}
	c.tx = &Tx{c: c}
	err := c.setAutoCommitAttr(api.SQL_AUTOCOMMIT_OFF)
	if err != nil {
		c.bad.Store(true)
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
		c.bad.Store(true)
		return c.newError("SQLEndTran", c.h)
	}
	c.tx = nil
	err := c.setAutoCommitAttr(api.SQL_AUTOCOMMIT_ON)
	if err != nil {
		c.bad.Store(true)
		return err
	}
	return nil
}