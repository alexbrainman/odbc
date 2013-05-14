// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

const (
	SQL_OV_ODBC3 = 3

	SQL_ATTR_ODBC_VERSION = 200

	SQL_DRIVER_NOPROMPT = 0

	SQL_HANDLE_ENV  = 1
	SQL_HANDLE_DBC  = 2
	SQL_HANDLE_STMT = 3

	SQL_SUCCESS            = 0
	SQL_SUCCESS_WITH_INFO  = 1
	SQL_INVALID_HANDLE     = -2
	SQL_NO_DATA            = 100
	SQL_NO_TOTAL           = -4
	SQL_NTS                = -3
	SQL_MAX_MESSAGE_LENGTH = 512
	SQL_NULL_HANDLE        = 0
	SQL_NULL_HENV          = 0
	SQL_NULL_HDBC          = 0
	SQL_NULL_HSTMT         = 0

	SQL_PARAM_INPUT = 1

	SQL_NULL_DATA    = -1
	SQL_DATA_AT_EXEC = -2

	SQL_UNKNOWN_TYPE    = 0
	SQL_CHAR            = 1
	SQL_NUMERIC         = 2
	SQL_DECIMAL         = 3
	SQL_INTEGER         = 4
	SQL_SMALLINT        = 5
	SQL_FLOAT           = 6
	SQL_REAL            = 7
	SQL_DOUBLE          = 8
	SQL_DATETIME        = 9
	SQL_DATE            = 9
	SQL_TIME            = 10
	SQL_VARCHAR         = 12
	SQL_TYPE_DATE       = 91
	SQL_TYPE_TIME       = 92
	SQL_TYPE_TIMESTAMP  = 93
	SQL_TIMESTAMP       = 11
	SQL_LONGVARCHAR     = -1
	SQL_BINARY          = -2
	SQL_VARBINARY       = -3
	SQL_LONGVARBINARY   = -4
	SQL_BIGINT          = -5
	SQL_TINYINT         = -6
	SQL_BIT             = -7
	SQL_WCHAR           = -8
	SQL_WVARCHAR        = -9
	SQL_WLONGVARCHAR    = -10
	SQL_GUID            = -11
	SQL_SIGNED_OFFSET   = -20
	SQL_UNSIGNED_OFFSET = -22
	SQL_SS_XML          = -152

	SQL_C_CHAR           = SQL_CHAR
	SQL_C_LONG           = SQL_INTEGER
	SQL_C_SHORT          = SQL_SMALLINT
	SQL_C_FLOAT          = SQL_REAL
	SQL_C_DOUBLE         = SQL_DOUBLE
	SQL_C_NUMERIC        = SQL_NUMERIC
	SQL_C_DATE           = SQL_DATE
	SQL_C_TIME           = SQL_TIME
	SQL_C_TYPE_TIMESTAMP = SQL_TYPE_TIMESTAMP
	SQL_C_TIMESTAMP      = SQL_TIMESTAMP
	SQL_C_BINARY         = SQL_BINARY
	SQL_C_BIT            = SQL_BIT
	SQL_C_WCHAR          = SQL_WCHAR
	SQL_C_DEFAULT        = 99
	SQL_C_SBIGINT        = SQL_BIGINT + SQL_SIGNED_OFFSET
	SQL_C_UBIGINT        = SQL_BIGINT + SQL_UNSIGNED_OFFSET
	SQL_C_GUID           = SQL_GUID

	SQL_COMMIT   = 0
	SQL_ROLLBACK = 1

	SQL_AUTOCOMMIT         = 102
	SQL_ATTR_AUTOCOMMIT    = SQL_AUTOCOMMIT
	SQL_AUTOCOMMIT_OFF     = 0
	SQL_AUTOCOMMIT_ON      = 1
	SQL_AUTOCOMMIT_DEFAULT = SQL_AUTOCOMMIT_ON

	SQL_IS_UINTEGER = -5

	//Connection pooling
	SQL_ATTR_CONNECTION_POOLING = 201
	SQL_ATTR_CP_MATCH           = 202
	SQL_CP_OFF                  = 0
	SQL_CP_ONE_PER_DRIVER       = 1
	SQL_CP_ONE_PER_HENV         = 2
	SQL_CP_DEFAULT              = SQL_CP_OFF
	SQL_CP_STRICT_MATCH         = 0
	SQL_CP_RELAXED_MATCH        = 1
)

type (
	SQLHANDLE uintptr
	SQLHENV   SQLHANDLE
	SQLHDBC   SQLHANDLE
	SQLHSTMT  SQLHANDLE
	SQLHWND   uintptr

	SQLWCHAR     uint16
	SQLSCHAR     int8
	SQLSMALLINT  int16
	SQLUSMALLINT uint16
	SQLINTEGER   int32
	SQLUINTEGER  uint32
	SQLPOINTER   uintptr
	SQLRETURN    SQLSMALLINT

	SQLGUID struct {
		Data1 uint32
		Data2 uint16
		Data3 uint16
		Data4 [8]byte
	}
)
