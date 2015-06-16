// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"fmt"

	"github.com/alexbrainman/odbc/api"
)

func ToHandleAndType(handle interface{}) (h api.SQLHANDLE, ht api.SQLSMALLINT) {
	switch v := handle.(type) {
	case api.SQLHENV:
		if v == api.SQLHENV(api.SQL_NULL_HANDLE) {
			ht = 0
		} else {
			ht = api.SQL_HANDLE_ENV
		}
		h = api.SQLHANDLE(v)
	case api.SQLHDBC:
		ht = api.SQL_HANDLE_DBC
		h = api.SQLHANDLE(v)
	case api.SQLHSTMT:
		ht = api.SQL_HANDLE_STMT
		h = api.SQLHANDLE(v)
	default:
		panic(fmt.Errorf("unexpected handle type %T", v))
	}
	return h, ht
}

func releaseHandle(handle interface{}) error {
	h, ht := ToHandleAndType(handle)
	ret := api.SQLFreeHandle(ht, h)
	if ret == api.SQL_INVALID_HANDLE {
		return fmt.Errorf("SQLFreeHandle(%d, %d) returns SQL_INVALID_HANDLE", ht, h)
	}
	if IsError(ret) {
		return NewError("SQLFreeHandle", handle)
	}
	drv.Stats.updateHandleCount(ht, -1)
	return nil
}
