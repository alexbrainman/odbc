// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"errors"

	"github.com/alexbrainman/odbc/api"
)

var ErrTXCompleted = errors.New("transaction already completed")

type Tx struct {
	c    *Conn
	opts driver.TxOptions
}

// implement driver.Tx
func (tx *Tx) Commit() error {
	return tx.endTx(api.SQL_COMMIT)
}

// implement driver.Tx
func (tx *Tx) Rollback() error {
	return tx.endTx(api.SQL_ROLLBACK)
}

func (tx *Tx) endTx(mode api.SQLSMALLINT) error {
	if tx.c.tx == nil {
		return ErrTXCompleted
	}
	tx.c.tx = nil
	if ret := api.SQLEndTran(api.SQL_HANDLE_DBC, api.SQLHANDLE(tx.c.h), mode); IsError(ret) {
		tx.c.bad.Store(true)
		return tx.c.newError("SQLEndTran", tx.c.h)
	}

	if ret := api.SQLSetConnectUIntPtrAttr(tx.c.h, api.SQL_ATTR_AUTOCOMMIT, api.SQL_AUTOCOMMIT_ON, api.SQL_IS_UINTEGER); IsError(ret) {
		tx.c.bad.Store(true)
		return tx.c.newError("SQLSetConnectUIntPtrAttr", tx.c.h)
	}

	if tx.opts.ReadOnly {
		if ret := api.SQLSetConnectUIntPtrAttr(tx.c.h, api.SQL_ATTR_ACCESS_MODE, api.SQL_MODE_READ_WRITE, api.SQL_IS_UINTEGER); IsError(ret) {
			tx.c.bad.Store(true)
			return NewError("SQLSetConnectUIntPtrAttr", tx.c.h)
		}
	}
	return nil
}
