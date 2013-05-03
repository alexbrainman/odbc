// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"code.google.com/p/odbc/api"
	"database/sql/driver"
	"fmt"
	"runtime"
	"time"
	"unsafe"
)

type Parameter struct {
	SQLType     api.SQLSMALLINT
	Decimal     api.SQLSMALLINT
	Size        api.SQLULEN
	Data        interface{} // to keep data away from gc
	isDescribed bool
}

func (p *Parameter) BindValue(h api.SQLHSTMT, idx int, v driver.Value) error {
	var ctype, sqltype, decimal api.SQLSMALLINT
	var size api.SQLULEN
	var buflen, plen api.SQLLEN
	var buf unsafe.Pointer
	switch d := v.(type) {
	case nil:
		var b byte
		ctype = api.SQL_C_BIT
		p.Data = &b
		buf = unsafe.Pointer(&b)
		plen = api.SQL_NULL_DATA
		sqltype = api.SQL_BIT
		size = 1
	case string:
		ctype = api.SQL_C_WCHAR
		b := api.StringToUTF16(d)
		p.Data = &b[0]
		buf = unsafe.Pointer(&b[0])
		l := len(b)
		l -= 1 // remove terminating 0
		size = api.SQLULEN(l)
		l *= 2 // every char takes 2 bytes
		buflen = api.SQLLEN(l)
		plen = buflen
		sqltype = api.SQL_WCHAR
	case int64:
		ctype = api.SQL_C_SBIGINT
		p.Data = &d
		buf = unsafe.Pointer(&d)
		sqltype = api.SQL_BIGINT
	case bool:
		var b byte
		if d {
			b = 1
		}
		ctype = api.SQL_C_BIT
		p.Data = &b
		buf = unsafe.Pointer(&b)
		sqltype = api.SQL_BIT
	case float64:
		ctype = api.SQL_C_DOUBLE
		p.Data = &d
		buf = unsafe.Pointer(&d)
		sqltype = api.SQL_DOUBLE
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
		size = 23 // 20 + s (the number of characters in the yyyy-mm-dd hh:mm:ss[.fff...] format, where s is the seconds precision).
	case []byte:
		ctype = api.SQL_C_BINARY
		b := make([]byte, len(d))
		copy(b, d)
		p.Data = &b[0]
		buf = unsafe.Pointer(&b[0])
		buflen = api.SQLLEN(len(b))
		plen = buflen
		size = api.SQLULEN(len(b))
		sqltype = api.SQL_BINARY
	default:
		panic(fmt.Errorf("unsupported type %T", v))
	}
	if p.isDescribed {
		sqltype = p.SQLType
		decimal = p.Decimal
		size = p.Size
	}
	ret := api.SQLBindParameter(h, api.SQLUSMALLINT(idx+1),
		api.SQL_PARAM_INPUT, ctype, sqltype, size, decimal,
		api.SQLPOINTER(buf), buflen, &plen)
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
	if runtime.GOOS == "windows" {
		// SQLDescribeParam is not implemented by freedts
		for i := range ps {
			p := &ps[i]
			ret = api.SQLDescribeParam(h, api.SQLUSMALLINT(i+1),
				&p.SQLType, &p.Size, &p.Decimal, &nullable)
			if IsError(ret) {
				return nil, NewError("SQLDescribeParam", h)
			}
			p.isDescribed = true
		}
	}
	return ps, nil
}
