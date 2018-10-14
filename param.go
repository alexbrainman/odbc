// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"fmt"
	"time"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

type Parameter struct {
	SQLType     api.SQLSMALLINT
	Decimal     api.SQLSMALLINT
	Size        api.SQLULEN
	isDescribed bool
	// Following fields store data used later by SQLExecute.
	// The fields keep data alive and away from gc.
	Data             interface{}
	StrLen_or_IndPtr api.SQLLEN
}

// StoreStrLen_or_IndPtr stores v into StrLen_or_IndPtr field of p
// and returns address of that field.
func (p *Parameter) StoreStrLen_or_IndPtr(v api.SQLLEN) *api.SQLLEN {
	p.StrLen_or_IndPtr = v
	return &p.StrLen_or_IndPtr

}

func (p *Parameter) BindValue(h api.SQLHSTMT, idx int, v driver.Value, conn *Conn) error {
	// TODO(brainman): Reuse memory for previously bound values. If memory
	// is reused, we, probably, do not need to call SQLBindParameter either.
	var ctype, sqltype, decimal api.SQLSMALLINT
	var size api.SQLULEN
	var buflen api.SQLLEN
	var plen *api.SQLLEN
	var buf unsafe.Pointer
	switch d := v.(type) {
	case nil:
		ctype = api.SQL_C_WCHAR
		p.Data = nil
		buf = nil
		size = 1
		buflen = 0
		plen = p.StoreStrLen_or_IndPtr(api.SQL_NULL_DATA)
		sqltype = api.SQL_WCHAR
	case string:
		ctype = api.SQL_C_WCHAR
		b := api.StringToUTF16(d)
		p.Data = b
		buf = unsafe.Pointer(&b[0])
		l := len(b)
		l -= 1 // remove terminating 0
		size = api.SQLULEN(l)
		if size < 1 {
			// size cannot be less then 1 even for empty fields
			size = 1
		}
		l *= 2 // every char takes 2 bytes
		buflen = api.SQLLEN(l)
		plen = p.StoreStrLen_or_IndPtr(buflen)
		if !conn.isMSAccessDriver {
			switch {
			case size >= 4000:
				sqltype = api.SQL_WLONGVARCHAR
			case p.isDescribed:
				sqltype = p.SQLType
			case size <= 1:
				sqltype = api.SQL_WVARCHAR
			default:
				sqltype = api.SQL_WCHAR
			}
		} else {
			// MS Acess requires SQL_WLONGVARCHAR for MEMO.
			// https://docs.microsoft.com/en-us/sql/odbc/microsoft/microsoft-access-data-types
			sqltype = api.SQL_WLONGVARCHAR
		}
	case int64:
		if -0x80000000 < d && d < 0x7fffffff {
			// Some ODBC drivers do not support SQL_BIGINT.
			// Use SQL_INTEGER if the value fit in int32.
			// See issue #78 for details.
			d2 := int32(d)
			ctype = api.SQL_C_LONG
			p.Data = &d2
			buf = unsafe.Pointer(&d2)
			sqltype = api.SQL_INTEGER
			size = 4
		} else {
			ctype = api.SQL_C_SBIGINT
			p.Data = &d
			buf = unsafe.Pointer(&d)
			sqltype = api.SQL_BIGINT
			size = 8
		}
	case bool:
		var b byte
		if d {
			b = 1
		}
		ctype = api.SQL_C_BIT
		p.Data = &b
		buf = unsafe.Pointer(&b)
		sqltype = api.SQL_BIT
		size = 1
	case float64:
		ctype = api.SQL_C_DOUBLE
		p.Data = &d
		buf = unsafe.Pointer(&d)
		sqltype = api.SQL_DOUBLE
		size = 8
	case time.Time:
		ctype = api.SQL_C_TYPE_TIMESTAMP
		y, m, day := d.Date()
		b := api.SQL_TIMESTAMP_STRUCT{
			Year:     api.SQLSMALLINT(y),
			Month:    api.SQLUSMALLINT(m),
			Day:      api.SQLUSMALLINT(day),
			Hour:     api.SQLUSMALLINT(d.Hour()),
			Minute:   api.SQLUSMALLINT(d.Minute()),
			Second:   api.SQLUSMALLINT(d.Second()),
			Fraction: api.SQLUINTEGER(d.Nanosecond()),
		}
		p.Data = &b
		buf = unsafe.Pointer(&b)
		sqltype = api.SQL_TYPE_TIMESTAMP
		if p.isDescribed && p.SQLType == api.SQL_TYPE_TIMESTAMP {
			decimal = p.Decimal
		}
		if decimal <= 0 {
			// represented as yyyy-mm-dd hh:mm:ss.fff format in ms sql server
			decimal = 3
		}
		size = 20 + api.SQLULEN(decimal)
	case []byte:
		ctype = api.SQL_C_BINARY
		b := make([]byte, len(d))
		copy(b, d)
		p.Data = b
		buf = unsafe.Pointer(&b[0])
		buflen = api.SQLLEN(len(b))
		plen = p.StoreStrLen_or_IndPtr(buflen)
		size = api.SQLULEN(len(b))
		switch {
		case p.isDescribed:
			sqltype = p.SQLType
		case size <= 0:
			sqltype = api.SQL_LONGVARBINARY
		case size >= 8000:
			sqltype = api.SQL_LONGVARBINARY
		default:
			sqltype = api.SQL_BINARY
		}
	default:
		return fmt.Errorf("unsupported type %T", v)
	}
	ret := api.SQLBindParameter(h, api.SQLUSMALLINT(idx+1),
		api.SQL_PARAM_INPUT, ctype, sqltype, size, decimal,
		api.SQLPOINTER(buf), buflen, plen)
	if IsError(ret) {
		return NewError("SQLBindParameter", h)
	}
	return nil
}

func ExtractParameters(h api.SQLHSTMT) ([]Parameter, error) {
	// count parameters
	var n, nullable api.SQLSMALLINT
	ret := api.SQLNumParams(h, &n)
	if IsError(ret) {
		return nil, NewError("SQLNumParams", h)
	}
	if n <= 0 {
		// no parameters
		return nil, nil
	}
	ps := make([]Parameter, n)
	// fetch param descriptions
	for i := range ps {
		p := &ps[i]
		ret = api.SQLDescribeParam(h, api.SQLUSMALLINT(i+1),
			&p.SQLType, &p.Size, &p.Decimal, &nullable)
		if IsError(ret) {
			// SQLDescribeParam is not implemented by freedts,
			// it even fails for some statements on windows.
			// Will try request without these descriptions
			continue
		}
		p.isDescribed = true
		// SQL Server MAX types (varchar(max), nvarchar(max),
		// varbinary(max) are identified by size = 0
		if p.Size == 0 {
			switch p.SQLType {
			case api.SQL_VARBINARY:
				p.SQLType = api.SQL_LONGVARBINARY
			case api.SQL_VARCHAR:
				p.SQLType = api.SQL_LONGVARCHAR
			case api.SQL_WVARCHAR:
				p.SQLType = api.SQL_WLONGVARCHAR
			}
		}
	}
	return ps, nil
}
