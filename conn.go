// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"syscall"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

type Conn struct {
	h   api.SQLHDBC
	tx  *Tx
	bad bool
}

func (d *Driver) Open(dsn string) (driver.Conn, error) {
	var out api.SQLHANDLE
	ret := api.SQLAllocHandle(api.SQL_HANDLE_DBC, api.SQLHANDLE(d.h), &out)
	if IsError(ret) {
		return nil, NewError("SQLAllocHandle", d.h)
	}
	h := api.SQLHDBC(out)
	drv.Stats.updateHandleCount(api.SQL_HANDLE_DBC, 1)

	b := syscall.StringByteSlice(dsn)
	ret = api.SQLDriverConnect(h, 0,
		(*api.SQLCHAR)(unsafe.Pointer(&b[0])), api.SQL_NTS,
		nil, 0, nil, api.SQL_DRIVER_NOPROMPT)
	if IsError(ret) {
		defer releaseHandle(h)
		return nil, NewError("SQLDriverConnect", h)
	}
	return &Conn{h: h}, nil
}

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
		c.bad = true
	}
	return err
}
