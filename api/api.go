// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"unicode/utf16"
)

type (
	SQL_DATE_STRUCT struct {
		Year  SQLSMALLINT
		Month SQLUSMALLINT
		Day   SQLUSMALLINT
	}

	SQL_TIMESTAMP_STRUCT struct {
		Year     SQLSMALLINT
		Month    SQLUSMALLINT
		Day      SQLUSMALLINT
		Hour     SQLUSMALLINT
		Minute   SQLUSMALLINT
		Second   SQLUSMALLINT
		Fraction SQLUINTEGER
	}
)

//sys	SQLAllocHandle(handleType SQLSMALLINT, inputHandle SQLHANDLE, outputHandle *SQLHANDLE) (ret SQLRETURN) = odbc32.SQLAllocHandle
//sys	SQLBindCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr SQLPOINTER, bufferLength SQLLEN, vallen *SQLLEN) (ret SQLRETURN) = odbc32.SQLBindCol
//sys	SQLBindParameter(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, inputOutputType SQLSMALLINT, valueType SQLSMALLINT, parameterType SQLSMALLINT, columnSize SQLULEN, decimalDigits SQLSMALLINT, parameterValue SQLPOINTER, bufferLength SQLLEN, ind *SQLLEN) (ret SQLRETURN) = odbc32.SQLBindParameter
//sys	SQLCloseCursor(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLCloseCursor
//sys	SQLDescribeCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, columnName *SQLWCHAR, bufferLength SQLSMALLINT, nameLengthPtr *SQLSMALLINT, dataTypePtr *SQLSMALLINT, columnSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLDescribeColW
//sys	SQLDescribeParam(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, dataTypePtr *SQLSMALLINT, parameterSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLDescribeParam
//sys	SQLDisconnect(connectionHandle SQLHDBC) (ret SQLRETURN) = odbc32.SQLDisconnect
//sys	SQLDriverConnect(connectionHandle SQLHDBC, windowHandle SQLHWND, inConnectionString *SQLWCHAR, stringLength1 SQLSMALLINT, outConnectionString *SQLWCHAR, bufferLength SQLSMALLINT, stringLength2Ptr *SQLSMALLINT, driverCompletion SQLUSMALLINT) (ret SQLRETURN) = odbc32.SQLDriverConnectW
//sys	SQLEndTran(handleType SQLSMALLINT, handle SQLHANDLE, completionType SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLEndTran
//sys	SQLExecute(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLExecute
//sys	SQLFetch(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLFetch
//sys	SQLFreeHandle(handleType SQLSMALLINT, handle SQLHANDLE) (ret SQLRETURN) = odbc32.SQLFreeHandle
//sys	SQLGetData(statementHandle SQLHSTMT, colOrParamNum SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr SQLPOINTER, bufferLength SQLLEN, vallen *SQLLEN) (ret SQLRETURN) = odbc32.SQLGetData
//sys	SQLGetDiagRec(handleType SQLSMALLINT, handle SQLHANDLE, recNumber SQLSMALLINT, sqlState *SQLWCHAR, nativeErrorPtr *SQLINTEGER, messageText *SQLWCHAR, bufferLength SQLSMALLINT, textLengthPtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLGetDiagRecW
//sys	SQLNumParams(statementHandle SQLHSTMT, parameterCountPtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLNumParams
//sys	SQLNumResultCols(statementHandle SQLHSTMT, columnCountPtr *SQLSMALLINT)  (ret SQLRETURN) = odbc32.SQLNumResultCols
//sys	SQLPrepare(statementHandle SQLHSTMT, statementText *SQLWCHAR, textLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLPrepareW
//sys	SQLRowCount(statementHandle SQLHSTMT, rowCountPtr *SQLLEN) (ret SQLRETURN) = odbc32.SQLRowCount
//sys	SQLSetEnvAttr(environmentHandle SQLHENV, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLSetEnvAttr
//sys	SQLSetConnectAttr(connectionHandle SQLHDBC, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLSetConnectAttrW

// UTF16ToString returns the UTF-8 encoding of the UTF-16 sequence s,
// with a terminating NUL removed.
func UTF16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			s = s[0:i]
			break
		}
	}
	return string(utf16.Decode(s))
}

// StringToUTF16 returns the UTF-16 encoding of the UTF-8 string s,
// with a terminating NUL added.
func StringToUTF16(s string) []uint16 { return utf16.Encode([]rune(s + "\x00")) }

// StringToUTF16Ptr returns pointer to the UTF-16 encoding of
// the UTF-8 string s, with a terminating NUL added.
func StringToUTF16Ptr(s string) *uint16 { return &StringToUTF16(s)[0] }
