// Copyright 2012 The Go Authors. All rights reserved.

// Use of this source code is governed by a BSD-style

// license that can be found in the LICENSE file.

package odbc

import (
	"fmt"
	"testing"
	"unsafe"
)

func TestBufferLen_IsNull(t *testing.T) {
	tests := []struct {
		name string
		i    interface{}
		l    BufferLen
		want bool
	}{
		// TODO: Add test cases.
		{name: "IsNull ", i: int64(-1), want: true},
		{name: "IsNull ", i: int32(-1), want: true},
		{name: "IsNull ", i: int64(0x00000000ffffffff), want: true},

		{name: "NotNull ", i: int32(1)},
		{name: "NotNull ", i: int32(0x7fffffff)},
		{name: "NotNull ", i: int64(0x1ffffffff)},
	}
	for _, tt := range tests {
		switch i := tt.i.(type) {
		case int64:
			if unsafe.Sizeof(tt.l) != 8 {
				continue
			}
			tt.l = BufferLen(i)
			tt.name += fmt.Sprintf("0x%016x", uint64(i))
		case int32:
			tt.l = BufferLen(i)
			tt.name += fmt.Sprintf("0x%08x", uint32(i))
		}
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.IsNull(); got != tt.want {
				t.Errorf("BufferLen.IsNull() = %v, want %v", got, tt.want)
			}
		})
	}
}
