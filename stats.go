// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"fmt"

	"github.com/alexbrainman/odbc/api"
	"go.uber.org/atomic"
)

type Stats struct {
	EnvCount  *atomic.Int32
	ConnCount *atomic.Int32
	StmtCount *atomic.Int32
}

func (s *Stats) updateHandleCount(handleType api.SQLSMALLINT, change int32) error {
	switch handleType {
	case api.SQL_HANDLE_ENV:
		s.EnvCount.Add(change)
	case api.SQL_HANDLE_DBC:
		s.ConnCount.Add(change)
	case api.SQL_HANDLE_STMT:
		s.StmtCount.Add(change)
	default:
		return fmt.Errorf("unexpected handle type %d", handleType)
	}
	return nil
}
