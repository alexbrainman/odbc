// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package odbc implements database/sql driver to access data via odbc interface.
//
package odbc

import (
	"database/sql"

	"github.com/alexbrainman/odbc/api"
)

const (
	DriverPoolModeNone  DriverPoolMode = 0
	DriverPoolModeBasic DriverPoolMode = 1
	DriverPoolModeFull  DriverPoolMode = 2
)

type DriverPoolMode int

var drv Driver

type Driver struct {
	Stats
	h        api.SQLHENV // environment handle
	initErr  error
	poolMode DriverPoolMode
}

func (d *Driver) PoolMode() DriverPoolMode {
	return d.poolMode
}

func (d *Driver) IsPooling() bool {
	return d.poolMode != DriverPoolModeNone
}

func (d *Driver) IsFullPooling() bool {
	return d.poolMode == DriverPoolModeFull
}

func (d *Driver) Close() error {
	// TODO(brainman): who will call (*Driver).Close (to dispose all opened handles)?
	h := d.h
	d.h = api.SQLHENV(api.SQL_NULL_HENV)
	return releaseHandle(h)
}

func initDriver() error {

	//TODO: find a way to make this attribute changeable at runtime
	//Enable connection pooling (this should be executed before allocating the environment handle)
	ret := api.SQLSetEnvUIntPtrAttr(api.SQLHENV(api.SQL_NULL_HENV), api.SQL_ATTR_CONNECTION_POOLING, api.SQL_CP_ONE_PER_HENV, api.SQL_IS_UINTEGER)
	if IsError(ret) {
		drv.poolMode = DriverPoolModeNone
	} else {
		drv.poolMode = DriverPoolModeBasic
	}

	//Allocate environment handle
	var out api.SQLHANDLE
	in := api.SQLHANDLE(api.SQL_NULL_HANDLE)
	ret = api.SQLAllocHandle(api.SQL_HANDLE_ENV, in, &out)
	if IsError(ret) {
		return NewError("SQLAllocHandle", api.SQLHENV(in))
	}
	drv.h = api.SQLHENV(out)
	err := drv.Stats.updateHandleCount(api.SQL_HANDLE_ENV, 1)
	if err != nil {
		drv.Close()
		return err
	}

	// will use ODBC v3
	ret = api.SQLSetEnvUIntPtrAttr(drv.h, api.SQL_ATTR_ODBC_VERSION, api.SQL_OV_ODBC3, 0)
	if IsError(ret) {
		defer drv.Close()
		return NewError("SQLSetEnvUIntPtrAttr(SQL_ATTR_ODBC_VERSION, SQL_OV_ODBC3)", drv.h)
	}

	if drv.IsPooling() {
		//Set relaxed connection pool matching
		ret = api.SQLSetEnvUIntPtrAttr(drv.h, api.SQL_ATTR_CP_MATCH, api.SQL_CP_RELAXED_MATCH, api.SQL_IS_UINTEGER)
		if !IsError(ret) {
			drv.poolMode = DriverPoolModeFull
		}
	}

	//TODO: it would be nice if we could call "drv.SetMaxIdleConns(0)" here but from the docs it looks like
	//the user must call this function after db.Open

	return nil
}

func init() {
	err := initDriver()
	if err != nil {
		drv.initErr = err
	}
	sql.Register("odbc", &drv)
}
