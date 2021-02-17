package odbc

import (
	"context"
	"database/sql/driver"
)

//implement driver.ConnBeginTx
func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.ctx = ctx
	//TODO(ninthclowd): set tx options
	return c.Begin()
}

//implement driver.ConnPrepareContext
func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	c.ctx = ctx
	return c.Prepare(query)
}

//implement driver.ExecerContext
func (c *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (result driver.Result, err error) {
	stmt, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	result, err = stmt.(*Stmt).Exec(toValues(args))
	_ = stmt.Close()
	return

}

//implement driver.QueryerContext
func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (rows driver.Rows, err error) {
	stmt, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	rows, err = stmt.(*Stmt).Query(toValues(args))
	_ = stmt.Close() //TODO(ninthclowd): should we be closing this here or let the user do it with rows
	return
}
