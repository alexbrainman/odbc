package odbc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

var ErrTXAlreadyStarted = errors.New("already in a transaction")

var sqlIsolationLevel = map[sql.IsolationLevel]uintptr{
	sql.LevelReadCommitted:   api.SQL_TXN_READ_COMMITTED,
	sql.LevelReadUncommitted: api.SQL_TXN_READ_UNCOMMITTED,
	sql.LevelRepeatableRead:  api.SQL_TXN_REPEATABLE_READ,
	sql.LevelSerializable:    api.SQL_TXN_SERIALIZABLE,
}

var testBeginErr error // used during tests

//implement driver.ConnBeginTx
func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	if c.bad.Load() {
		return nil, driver.ErrBadConn
	}
	//TODO(ninthclowd): refactor to use mocks / test hook functions or behavior tests so there isn't test logic in production
	if testBeginErr != nil {
		c.bad.Store(true)
		return nil, testBeginErr
	}

	if c.tx != nil {
		return nil, ErrTXAlreadyStarted
	}
	c.tx = &Tx{c: c, opts: opts}

	if ret := api.SQLSetConnectUIntPtrAttr(c.h, api.SQL_ATTR_AUTOCOMMIT, api.SQL_AUTOCOMMIT_OFF, api.SQL_IS_UINTEGER); IsError(ret) {
		c.bad.Store(true)
		return nil, NewError("SQLSetConnectUIntPtrAttr", c.h)
	}

	if isolation, modeAvailable := sqlIsolationLevel[sql.IsolationLevel(opts.Isolation)]; modeAvailable {
		if ret := api.SQLSetConnectUIntPtrAttr(c.h, api.SQL_ATTR_TXN_ISOLATION, isolation, api.SQL_IS_UINTEGER); IsError(ret) {
			c.bad.Store(true)
			return nil, NewError("SQLSetConnectUIntPtrAttr", c.h)
		}
	}
	if opts.ReadOnly {
		if ret := api.SQLSetConnectUIntPtrAttr(c.h, api.SQL_ATTR_ACCESS_MODE, api.SQL_MODE_READ_ONLY, api.SQL_IS_UINTEGER); IsError(ret) {
			c.bad.Store(true)
			return nil, NewError("SQLSetConnectUIntPtrAttr", c.h)
		}
	}

	return c.tx, nil
}

//implement driver.ConnPrepareContext
func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if c.bad.Load() {
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

	return &Stmt{
		c:          c,
		query:      query,
		h:          h,
		parameters: ps,
		rows:       nil,
	}, nil
}

//implement driver.ExecerContext
func (c *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (result driver.Result, err error) {
	//TODO(ninthclowd): build and execute a statement with SQLExecDirect instead of preparing the statement
	return nil, driver.ErrSkip
}

//implement driver.QueryerContext
func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (rows driver.Rows, err error) {
	//TODO(ninthclowd): build and execute a statement with SQLExecDirect instead of preparing the statement
	return nil, driver.ErrSkip
}

//implement driver.SessionResetter
func (c *Conn) ResetSession(ctx context.Context) error {
	if c.bad.Load() {
		return driver.ErrBadConn
	}
	return nil
}

//implement driver.Pinger
func (c *Conn) Ping(ctx context.Context) error {
	if c.bad.Load() {
		return driver.ErrBadConn
	}
	stmt, err := c.PrepareContext(ctx, ";")
	if err != nil {
		return driver.ErrBadConn
	}
	defer stmt.Close()

	if _, err := stmt.(*Stmt).ExecContext(ctx, nil); err != nil {
		return driver.ErrBadConn
	}
	return nil
}
