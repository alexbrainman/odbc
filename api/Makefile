# Copyright 2012 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

all: zapi_windows.go zapi_unix.go

zapi_windows.go: api.go
	GOOS=windows $(GOROOT)/src/pkg/syscall/mksyscall_windows.pl $^ \
		| gofmt \
		> $@

zapi_unix.go: api.go
	GOOS=linux ./mksyscall_unix.pl $^ \
		| gofmt \
		> $@
