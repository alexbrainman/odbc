// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"io"

	"github.com/alexbrainman/odbc/api"
)

type Rows struct {
	os *ODBCStmt
}

func (r *Rows) Columns() []string {
	names := make([]string, len(r.os.Cols))
	for i := 0; i < len(names); i++ {
		names[i] = r.os.Cols[i].Name()
	}
	return names
}

func (r *Rows) Next(dest []driver.Value) error {
	ret := api.SQLFetch(r.os.h)
	if ret == api.SQL_NO_DATA {
		return io.EOF
	}
	if IsError(ret) {
		return NewError("SQLFetch", r.os.h)
	}
	for i := range dest {
		v, err := r.os.Cols[i].Value(r.os.h, i)
		if err != nil {
			return err
		}
		dest[i] = v
	}
	return nil
}

func (r *Rows) Close() error {
	return r.os.closeByRows()
}

// ColumnTypeDatabaseTypeName return the database system type name.
func (r *Rows) ColumnTypeDatabaseTypeName(index int) string {
	switch x := r.os.Cols[index].(type) {
	case *BindableColumn:
		return cTypeString(x.CType)
	case *NonBindableColumn:
		return cTypeString(x.CType)
	}
	return ""
}

func cTypeString(ct api.SQLSMALLINT) string {
	switch ct {
	case api.SQL_C_CHAR:
		return "SQL_C_CHAR"
	case api.SQL_C_LONG:
		return "SQL_C_LONG"
	case api.SQL_C_SHORT:
		return "SQL_C_SHORT"
	case api.SQL_C_FLOAT:
		return "SQL_C_FLOAT"
	case api.SQL_C_DOUBLE:
		return "SQL_C_DOUBLE"
	case api.SQL_C_NUMERIC:
		return "SQL_C_NUMERIC"
	case api.SQL_C_DATE:
		return "SQL_C_DATE"
	case api.SQL_C_TIME:
		return "SQL_C_TIME"
	case api.SQL_C_TYPE_TIMESTAMP:
		return "SQL_C_TYPE_TIMESTAMP"
	case api.SQL_C_TIMESTAMP:
		return "SQL_C_TIMESTAMP"
	case api.SQL_C_BINARY:
		return "SQL_C_BINARY"
	case api.SQL_C_BIT:
		return "SQL_C_BIT"
	case api.SQL_C_WCHAR:
		return "SQL_C_WCHAR"
	case api.SQL_C_DEFAULT:
		return "SQL_C_DEFAULT"
	case api.SQL_C_SBIGINT:
		return "SQL_C_SBIGINT"
	case api.SQL_C_UBIGINT:
		return "SQL_C_UBIGINT"
	case api.SQL_C_GUID:
		return "SQL_C_GUID"
	}
	return ""
}
