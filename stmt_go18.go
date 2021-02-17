package odbc

import (
	"context"
	"database/sql/driver"
)

//implement driver.StmtExecContext
func (s *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	s.ctx = ctx
	return s.Exec(toValues(args))
}

//implement driver.StmtQueryContext
func (s *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	s.ctx = ctx
	return s.Query(toValues(args))

}

func toValues(args []driver.NamedValue) []driver.Value {
	values := make([]driver.Value, len(args))
	for _, arg := range args {
		values[arg.Ordinal-1] = arg.Value
	}
	return values
}
