// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"code.google.com/p/odbc/api"
	"fmt"
	"sync"
)

type Stats struct {
	EnvCount  int
	ConnCount int
	StmtCount int
	mu        sync.Mutex
}

func (s *Stats) updateHandleCount(handleType api.SQLSMALLINT, change int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch handleType {
	case api.SQL_HANDLE_ENV:
		s.EnvCount += change
	case api.SQL_HANDLE_DBC:
		s.ConnCount += change
	case api.SQL_HANDLE_STMT:
		s.StmtCount += change
	default:
		panic(fmt.Errorf("unexpected handle type %d", handleType))
	}
}
