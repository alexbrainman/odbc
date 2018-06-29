package odbc

import (
	"github.com/mpcjanssen/odbc/api"
	"unsafe"
)

type DSN struct {
	Name string
	Description string
}

func DataSources() ([]DSN, error) {
	//Allocate environment handle
	var out api.SQLHANDLE
	in := api.SQLHANDLE(api.SQL_NULL_HANDLE)
	ret := api.SQLAllocHandle(api.SQL_HANDLE_ENV, in, &out)
	if IsError(ret) {
		return nil, NewError("SQLAllocHandle", api.SQLHENV(in))
	}
	h := api.SQLHENV(out)
	err := drv.Stats.updateHandleCount(api.SQL_HANDLE_ENV, 1)
	if err != nil {
		return nil, err
	}
	defer releaseHandle(h)
	// will use ODBC v3
	ret = api.SQLSetEnvUIntPtrAttr(h, api.SQL_ATTR_ODBC_VERSION, api.SQL_OV_ODBC3, 0)
	if IsError(ret) {
		return nil, NewError("SQLSetEnvUIntPtrAttr", drv.h)
	}

	// Read DSNs

	var dsnName [api.SQL_MAX_DSN_LENGTH]byte
	var dsnNameLength api.SQLSMALLINT
	var dsnDescription [200]byte
	var dsnDescriptionLength api.SQLSMALLINT
	DSNs := make([]DSN, 0)
	for {
		ret = api.SQLDataSources(h, api.SQL_FETCH_NEXT,
			(*api.SQLSCHAR)(unsafe.Pointer(&dsnName[0])),
			api.SQLSMALLINT(len(dsnName)), &dsnNameLength,
			(*api.SQLSCHAR)(unsafe.Pointer(&dsnDescription[0])),
			api.SQLSMALLINT(len(dsnDescription)), &dsnDescriptionLength)
		if ret != api.SQL_SUCCESS {
			break
		}
		DSNs = append(DSNs, DSN{ string(dsnName[:dsnNameLength]), string(dsnDescription[:dsnDescriptionLength]) } )
	}
	return DSNs, nil
}
