package odbc

import (
	"context"
	"database/sql/driver"
)

type connector struct {
	d    *Driver
	name string
}

//implement driver.Connector
func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	panic("implement me")
}

//implement driver.Connector
func (c *connector) Driver() driver.Driver {
	panic("implement me")
}
