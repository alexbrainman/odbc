// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin linux freebsd
// +build cgo

package api

// #cgo darwin LDFLAGS: -lodbc
// #cgo linux LDFLAGS: -lodbc
// #cgo freebsd LDFLAGS: -L /usr/local/lib -lodbc
// #cgo freebsd CFLAGS: -I/usr/local/include
// #include <sql.h>
// #include <sqlext.h>
// #include <stdint.h>
/*
SQLRETURN sqlSetEnvUIntPtrAttr(SQLHENV environmentHandle, SQLINTEGER attribute, uintptr_t valuePtr, SQLINTEGER stringLength) {
	return SQLSetEnvAttr(environmentHandle, attribute, (SQLPOINTER)valuePtr, stringLength);
}

SQLRETURN sqlSetConnectUIntPtrAttr(SQLHDBC connectionHandle, SQLINTEGER attribute, uintptr_t valuePtr, SQLINTEGER stringLength) {
	return SQLSetConnectAttr(connectionHandle, attribute, (SQLPOINTER)valuePtr, stringLength);
}
*/
import "C"

const (
	SQL_OV_ODBC3 = uintptr(C.SQL_OV_ODBC3)

	SQL_ATTR_ODBC_VERSION = C.SQL_ATTR_ODBC_VERSION

	SQL_DRIVER_NOPROMPT = C.SQL_DRIVER_NOPROMPT

	SQL_HANDLE_ENV  = C.SQL_HANDLE_ENV
	SQL_HANDLE_DBC  = C.SQL_HANDLE_DBC
	SQL_HANDLE_STMT = C.SQL_HANDLE_STMT

	SQL_SUCCESS            = C.SQL_SUCCESS
	SQL_SUCCESS_WITH_INFO  = C.SQL_SUCCESS_WITH_INFO
	SQL_INVALID_HANDLE     = C.SQL_INVALID_HANDLE
	SQL_NO_DATA            = C.SQL_NO_DATA
	SQL_NO_TOTAL           = C.SQL_NO_TOTAL
	SQL_NTS                = C.SQL_NTS
	SQL_MAX_MESSAGE_LENGTH = C.SQL_MAX_MESSAGE_LENGTH
	SQL_NULL_HANDLE        = uintptr(C.SQL_NULL_HANDLE)
	SQL_NULL_HENV          = uintptr(C.SQL_NULL_HENV)
	SQL_NULL_HDBC          = uintptr(C.SQL_NULL_HDBC)
	SQL_NULL_HSTMT         = uintptr(C.SQL_NULL_HSTMT)

	SQL_PARAM_INPUT = C.SQL_PARAM_INPUT

	SQL_NULL_DATA    = C.SQL_NULL_DATA
	SQL_DATA_AT_EXEC = C.SQL_DATA_AT_EXEC

	SQL_UNKNOWN_TYPE    = C.SQL_UNKNOWN_TYPE
	SQL_CHAR            = C.SQL_CHAR
	SQL_NUMERIC         = C.SQL_NUMERIC
	SQL_DECIMAL         = C.SQL_DECIMAL
	SQL_INTEGER         = C.SQL_INTEGER
	SQL_SMALLINT        = C.SQL_SMALLINT
	SQL_FLOAT           = C.SQL_FLOAT
	SQL_REAL            = C.SQL_REAL
	SQL_DOUBLE          = C.SQL_DOUBLE
	SQL_DATETIME        = C.SQL_DATETIME
	SQL_DATE            = C.SQL_DATE
	SQL_TIME            = C.SQL_TIME
	SQL_VARCHAR         = C.SQL_VARCHAR
	SQL_TYPE_DATE       = C.SQL_TYPE_DATE
	SQL_TYPE_TIME       = C.SQL_TYPE_TIME
	SQL_TYPE_TIMESTAMP  = C.SQL_TYPE_TIMESTAMP
	SQL_TIMESTAMP       = C.SQL_TIMESTAMP
	SQL_LONGVARCHAR     = C.SQL_LONGVARCHAR
	SQL_BINARY          = C.SQL_BINARY
	SQL_VARBINARY       = C.SQL_VARBINARY
	SQL_LONGVARBINARY   = C.SQL_LONGVARBINARY
	SQL_BIGINT          = C.SQL_BIGINT
	SQL_TINYINT         = C.SQL_TINYINT
	SQL_BIT             = C.SQL_BIT
	SQL_WCHAR           = C.SQL_WCHAR
	SQL_WVARCHAR        = C.SQL_WVARCHAR
	SQL_WLONGVARCHAR    = C.SQL_WLONGVARCHAR
	SQL_GUID            = C.SQL_GUID
	SQL_SIGNED_OFFSET   = C.SQL_SIGNED_OFFSET
	SQL_UNSIGNED_OFFSET = C.SQL_UNSIGNED_OFFSET

	// TODO(lukemauldin): Not defined in sqlext.h. Using windows value, but it is not supported.
	SQL_SS_XML   = -152
	SQL_SS_TIME2 = -154

	SQL_C_CHAR           = C.SQL_C_CHAR
	SQL_C_LONG           = C.SQL_C_LONG
	SQL_C_SHORT          = C.SQL_C_SHORT
	SQL_C_FLOAT          = C.SQL_C_FLOAT
	SQL_C_DOUBLE         = C.SQL_C_DOUBLE
	SQL_C_NUMERIC        = C.SQL_C_NUMERIC
	SQL_C_DATE           = C.SQL_C_DATE
	SQL_C_TIME           = C.SQL_C_TIME
	SQL_C_TYPE_TIMESTAMP = C.SQL_C_TYPE_TIMESTAMP
	SQL_C_TIMESTAMP      = C.SQL_C_TIMESTAMP
	SQL_C_BINARY         = C.SQL_C_BINARY
	SQL_C_BIT            = C.SQL_C_BIT
	SQL_C_WCHAR          = C.SQL_C_WCHAR
	SQL_C_DEFAULT        = C.SQL_C_DEFAULT
	SQL_C_SBIGINT        = C.SQL_C_SBIGINT
	SQL_C_UBIGINT        = C.SQL_C_UBIGINT
	SQL_C_GUID           = C.SQL_C_GUID

	SQL_COMMIT   = C.SQL_COMMIT
	SQL_ROLLBACK = C.SQL_ROLLBACK

	SQL_AUTOCOMMIT         = C.SQL_AUTOCOMMIT
	SQL_ATTR_AUTOCOMMIT    = C.SQL_ATTR_AUTOCOMMIT
	SQL_AUTOCOMMIT_OFF     = C.SQL_AUTOCOMMIT_OFF
	SQL_AUTOCOMMIT_ON      = C.SQL_AUTOCOMMIT_ON
	SQL_AUTOCOMMIT_DEFAULT = C.SQL_AUTOCOMMIT_DEFAULT

	SQL_IS_UINTEGER = C.SQL_IS_UINTEGER

	//Connection pooling
	SQL_ATTR_CONNECTION_POOLING = C.SQL_ATTR_CONNECTION_POOLING
	SQL_ATTR_CP_MATCH           = C.SQL_ATTR_CP_MATCH
	SQL_CP_OFF                  = uintptr(C.SQL_CP_OFF)
	SQL_CP_ONE_PER_DRIVER       = uintptr(C.SQL_CP_ONE_PER_DRIVER)
	SQL_CP_ONE_PER_HENV         = uintptr(C.SQL_CP_ONE_PER_HENV)
	SQL_CP_DEFAULT              = SQL_CP_OFF
	SQL_CP_STRICT_MATCH         = uintptr(C.SQL_CP_STRICT_MATCH)
	SQL_CP_RELAXED_MATCH        = uintptr(C.SQL_CP_RELAXED_MATCH)
)

type (
	SQLHANDLE C.SQLHANDLE
	SQLHENV   C.SQLHENV
	SQLHDBC   C.SQLHDBC
	SQLHSTMT  C.SQLHSTMT
	SQLHWND   uintptr

	SQLWCHAR     C.SQLWCHAR
	SQLSCHAR     C.SQLSCHAR
	SQLSMALLINT  C.SQLSMALLINT
	SQLUSMALLINT C.SQLUSMALLINT
	SQLINTEGER   C.SQLINTEGER
	SQLUINTEGER  C.SQLUINTEGER
	SQLPOINTER   C.SQLPOINTER
	SQLRETURN    C.SQLRETURN

	SQLLEN  C.SQLLEN
	SQLULEN C.SQLULEN

	SQLGUID C.SQLGUID
)

func SQLSetEnvUIntPtrAttr(environmentHandle SQLHENV, attribute SQLINTEGER, valuePtr uintptr, stringLength SQLINTEGER) (ret SQLRETURN) {
	r := C.sqlSetEnvUIntPtrAttr(C.SQLHENV(environmentHandle), C.SQLINTEGER(attribute), C.uintptr_t(valuePtr), C.SQLINTEGER(stringLength))
	return SQLRETURN(r)
}

func SQLSetConnectUIntPtrAttr(connectionHandle SQLHDBC, attribute SQLINTEGER, valuePtr uintptr, stringLength SQLINTEGER) (ret SQLRETURN) {
	r := C.sqlSetConnectUIntPtrAttr(C.SQLHDBC(connectionHandle), C.SQLINTEGER(attribute), C.uintptr_t(valuePtr), C.SQLINTEGER(stringLength))
	return SQLRETURN(r)
}
