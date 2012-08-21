// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package odbc implements database/sql driver to access data via odbc interface.
//
package odbc

import (
	"code.google.com/p/odbc/api"
	"database/sql"
)

var drv Driver

type Driver struct {
	Stats
	h api.SQLHENV // environment handle
}

func initDriver() error {
	var out api.SQLHANDLE
	in := api.SQLHANDLE(api.SQL_NULL_HANDLE)
	ret := api.SQLAllocHandle(api.SQL_HANDLE_ENV, in, &out)
	if IsError(ret) {
		return NewError("SQLAllocHandle", api.SQLHENV(in))
	}
	drv.h = api.SQLHENV(out)
	drv.Stats.updateHandleCount(api.SQL_HANDLE_ENV, 1)

	// will use ODBC v3
	ret = api.SQLSetEnvAttr(drv.h, api.SQL_ATTR_ODBC_VERSION,
		api.SQLPOINTER(api.SQL_OV_ODBC3), 0)
	if IsError(ret) {
		defer releaseHandle(drv.h)
		return NewError("SQLSetEnvAttr", drv.h)
	}
	return nil
}

func (d *Driver) Close() error {
	// TODO(brainman): who will call (*Driver).Close (to dispose all opened handles)?
	h := d.h
	d.h = api.SQLHENV(api.SQL_NULL_HENV)
	return releaseHandle(h)
}

func init() {
	err := initDriver()
	if err != nil {
		panic(err)
	}
	sql.Register("odbc", &drv)
}
