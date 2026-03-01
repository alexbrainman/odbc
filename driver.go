// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package odbc implements database/sql driver to access data via odbc interface.
package odbc

import (
	"database/sql"

	"github.com/alexbrainman/odbc/api"
)

var drv Driver

// PoolingType controls ODBC-level connection pooling.
// Set this BEFORE any sql.Open call (typically in your own init()).
//
// Recommended: Leave as SQL_CP_OFF (default) and rely on Go's
// database/sql pool, which is goroutine-safe and well-tested.
// ODBC-level pooling (SQL_CP_ONE_PER_HENV) can cause thread-safety
// issues with unixODBC <= 2.3.9.
var PoolingType = api.SQL_CP_OFF

type Driver struct {
	Stats
	h       api.SQLHENV // environment handle
	initErr error
}

func initDriver() error {

	//Allocate environment handle
	var out api.SQLHANDLE
	in := api.SQLHANDLE(api.SQL_NULL_HANDLE)
	ret := api.SQLAllocHandle(api.SQL_HANDLE_ENV, in, &out)
	if IsError(ret) {
		return NewError("SQLAllocHandle", api.SQLHENV(in))
	}
	drv.h = api.SQLHENV(out)
	err := drv.Stats.updateHandleCount(api.SQL_HANDLE_ENV, 1)
	if err != nil {
		return err
	}

	// will use ODBC v3
	ret = api.SQLSetEnvUIntPtrAttr(drv.h, api.SQL_ATTR_ODBC_VERSION, api.SQL_OV_ODBC3, 0)
	if IsError(ret) {
		defer releaseHandle(drv.h)
		return NewError("SQLSetEnvUIntPtrAttr", drv.h)
	}

	// Connection pooling: configurable via the exported PoolingType var.
	// Default is SQL_CP_OFF to avoid double-pooling with Go's sql.DB.
	if PoolingType != api.SQL_CP_OFF {
		ret = api.SQLSetEnvUIntPtrAttr(drv.h, api.SQL_ATTR_CONNECTION_POOLING, PoolingType, api.SQL_IS_UINTEGER)
		if IsError(ret) {
			defer releaseHandle(drv.h)
			return NewError("SQLSetEnvUIntPtrAttr", drv.h)
		}

		ret = api.SQLSetEnvUIntPtrAttr(drv.h, api.SQL_ATTR_CP_MATCH, api.SQL_CP_RELAXED_MATCH, api.SQL_IS_UINTEGER)
		if IsError(ret) {
			defer releaseHandle(drv.h)
			return NewError("SQLSetEnvUIntPtrAttr", drv.h)
		}
	}

	//TODO: it would be nice if we could call "drv.SetMaxIdleConns(0)" here but from the docs it looks like
	//the user must call this function after db.Open

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
		drv.initErr = err
	}
	sql.Register("odbc", &drv)
}
