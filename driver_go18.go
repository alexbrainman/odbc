package odbc

import "database/sql/driver"

//implement driver.DriverContext
func (d *Driver) OpenConnector(name string) (driver.Connector, error) {
	return &connector{
		d:    d,
		name: name,
	}, nil
}
