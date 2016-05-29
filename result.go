// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"errors"
)

type Result struct {
	rowCount int64
}

func (r *Result) LastInsertId() (int64, error) {
	// TODO(brainman): implement (*Result).LastInsertId
	return 0, errors.New("not implemented")
}

func (r *Result) RowsAffected() (int64, error) {
	return r.rowCount, nil
}
