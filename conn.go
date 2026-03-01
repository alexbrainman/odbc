// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

type Conn struct {
	h                api.SQLHDBC
	tx               *Tx
	bad              bool
	isMSAccessDriver bool
	isInformixDriver bool
	dbmsName         string // populated at connect time via SQLGetInfo
}

var accessDriverSubstr = strings.ToUpper(strings.Replace("DRIVER={Microsoft Access Driver", " ", "", -1))

func (d *Driver) Open(dsn string) (driver.Conn, error) {
	if d.initErr != nil {
		return nil, d.initErr
	}

	var out api.SQLHANDLE
	ret := api.SQLAllocHandle(api.SQL_HANDLE_DBC, api.SQLHANDLE(d.h), &out)
	if IsError(ret) {
		return nil, NewError("SQLAllocHandle", d.h)
	}
	h := api.SQLHDBC(out)
	drv.Stats.updateHandleCount(api.SQL_HANDLE_DBC, 1)

	b := api.StringToUTF16(dsn)
	ret = api.SQLDriverConnect(h, 0,
		(*api.SQLWCHAR)(unsafe.Pointer(&b[0])), api.SQL_NTS,
		nil, 0, nil, api.SQL_DRIVER_NOPROMPT)
	if IsError(ret) {
		defer releaseHandle(h)
		return nil, NewError("SQLDriverConnect", h)
	}
	isAccess := strings.Contains(strings.ToUpper(strings.Replace(dsn, " ", "", -1)), accessDriverSubstr)

	// Detect DBMS name via SQLGetInfo for driver-specific handling.
	dbmsName := getDBMSName(h)
	isInformix := strings.Contains(strings.ToUpper(dbmsName), "INFORMIX")

	return &Conn{
		h:                h,
		isMSAccessDriver: isAccess,
		isInformixDriver: isInformix,
		dbmsName:         dbmsName,
	}, nil
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

// getDBMSName retrieves the DBMS product name via SQLGetInfo.
// Returns empty string on failure (non-fatal).
func getDBMSName(h api.SQLHDBC) string {
	buf := make([]uint16, 128)
	var outLen api.SQLSMALLINT
	ret := api.SQLGetInfo(h, api.SQL_DBMS_NAME,
		api.SQLPOINTER(unsafe.Pointer(&buf[0])),
		api.SQLSMALLINT(len(buf)*2),
		&outLen)
	if IsError(ret) {
		return ""
	}
	n := int(outLen) / 2
	if n > len(buf) {
		n = len(buf)
	}
	return api.UTF16ToString(buf[:n])
}

// Ping implements driver.Pinger.
// For Informix, uses SQL_ATTR_CONNECTION_DEAD which is cheaper than
// a round-trip query. Falls back to a lightweight query for other drivers.
func (c *Conn) Ping(ctx context.Context) error {
	if c.bad {
		return driver.ErrBadConn
	}

	// Check context first.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Try SQL_ATTR_CONNECTION_DEAD (supported by most modern drivers).
	var dead int32
	var outLen api.SQLINTEGER
	ret := api.SQLGetConnectAttr(c.h, api.SQL_ATTR_CONNECTION_DEAD,
		api.SQLPOINTER(unsafe.Pointer(&dead)),
		4, &outLen)
	if !IsError(ret) {
		if dead == api.SQL_CD_TRUE {
			c.bad = true
			return driver.ErrBadConn
		}
		return nil
	}

	// Fallback: execute a no-op query.
	_, err := c.QueryContext(ctx, "SELECT 1", nil)
	if err != nil {
		c.bad = true
		return driver.ErrBadConn
	}
	return nil
}

// IsValid implements driver.Validator.
// Called by sql.DB's pool to check if a connection can be reused.
func (c *Conn) IsValid() bool {
	if c.bad {
		return false
	}
	var dead int32
	var outLen api.SQLINTEGER
	ret := api.SQLGetConnectAttr(c.h, api.SQL_ATTR_CONNECTION_DEAD,
		api.SQLPOINTER(unsafe.Pointer(&dead)),
		4, &outLen)
	if IsError(ret) {
		// If we can't check, assume it's still valid.
		return true
	}
	return dead != api.SQL_CD_TRUE
}

// ResetSession implements driver.SessionResetter.
// Called by sql.DB when a connection is returned to the pool.
func (c *Conn) ResetSession(ctx context.Context) error {
	if c.bad {
		return driver.ErrBadConn
	}
	// If there's an orphaned transaction, roll it back.
	if c.tx != nil {
		c.tx.Rollback()
	}
	return nil
}

// QueryContext implements the driver.QueryerContext interface.
// As per the specifications, it honours the context timeout and returns when the context is cancelled.
// When the context is cancelled, it first cancels the statement, closes it, and then returns an error.
func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	// prepare the statement
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	os, err := c.PrepareODBCStmt(query)
	if err != nil {
		return nil, err
	}
	defer os.closeByStmt()

	// check if context is canceled before executing the query
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// execute the statement
	rowsChan := make(chan driver.Rows)
	errorChan := make(chan error)
	go func() {
		err := c.wrapQuery(ctx, os, dargs)
		if err != nil {
			errorChan <- err
			return
		}
		os.usedByRows = true
		rowsChan <- &Rows{os: os}
	}()
	return c.waitQuery(ctx, os, rowsChan, errorChan)
}

// wrapQuery is following the same logic as `stmt.Query()` except that we don't use a lock
// because the ODBC statement doesn't get exposed externally.
func (c *Conn) wrapQuery(ctx context.Context, os *ODBCStmt, dargs []driver.Value) error {
	if err := os.Exec(dargs, c); err != nil {
		return err
	}

	if err := os.BindColumns(); err != nil {
		return err
	}
	return nil
}

// waitQuery waits for either os rows or error to arrive from rowsChan and errorChan.
// waitQuery also waits for ctx to signal completion.
// The function returns received rows or the error.
func (c *Conn) waitQuery(ctx context.Context, os *ODBCStmt, rowsChan <-chan driver.Rows, errorChan <-chan error) (driver.Rows, error) {
	select {
	case <-ctx.Done():
		// context has been cancelled or has expired, cancel the statement and ignore the os.Cancel error
		os.Cancel()
		// the statement has been cancelled, the query execution should eventually succeed or fail now
		select {
		// ignore the ODBC error and return ctx.Err() instead
		case <-errorChan:
			return nil, ctx.Err()
		case rows := <-rowsChan:
			return rows, nil
		}
	case err := <-errorChan:
		return nil, err
	case rows := <-rowsChan:
		return rows, nil
	}
}

// namedValueToValue is a utility function that converts a driver.NamedValue into a driver.Value.
// Source:
// https://github.com/golang/go/blob/03ac39ce5e6af4c4bca58b54d5b160a154b7aa0e/src/database/sql/ctxutil.go#L137-L146
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	dargs := make([]driver.Value, len(named))
	for n, param := range named {
		if len(param.Name) > 0 {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		dargs[n] = param.Value
	}
	return dargs, nil
}
