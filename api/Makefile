# Copyright 2012 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

all: zapi_windows.go zapi_unix.go

TMP=/tmp/mksyscall_windows

zapi_windows.go: api.go
	go build -o $(TMP) $(GOROOT)/src/pkg/syscall/mksyscall_windows.go
	GOOS=windows $(TMP) $^ | gofmt > $@
	rm $(TMP)

zapi_unix.go: api.go
	./mksyscall_unix.pl $^ \
		| gofmt \
		> $@
