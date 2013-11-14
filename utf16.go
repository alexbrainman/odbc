// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"unicode/utf16"
	"unicode/utf8"
)

const (
	replacementChar = '\uFFFD' // Unicode replacement character

	// 0xd800-0xdc00 encodes the high 10 bits of a pair.
	// 0xdc00-0xe000 encodes the low 10 bits of a pair.
	// the value is those 20 bits plus 0x10000.
	surr1 = 0xd800
	surr2 = 0xdc00
	surr3 = 0xe000
)

// utf16toutf8 returns the UTF-8 encoding of the UTF-16 sequence s,
// with a terminating NUL removed.
func utf16toutf8(s []uint16) []byte {
	for i, v := range s {
		if v == 0 {
			s = s[0:i]
			break
		}
	}
	buf := make([]byte, 0, len(s)*2) // allow 2 bytes for every rune
	b := make([]byte, 4)
	for i := 0; i < len(s); i++ {
		var rr rune
		switch r := s[i]; {
		case surr1 <= r && r < surr2 && i+1 < len(s) &&
			surr2 <= s[i+1] && s[i+1] < surr3:
			// valid surrogate sequence
			rr = utf16.DecodeRune(rune(r), rune(s[i+1]))
			i++
		case surr1 <= r && r < surr3:
			// invalid surrogate sequence
			rr = replacementChar
		default:
			// normal rune
			rr = rune(r)
		}
		b := b[:cap(b)]
		n := utf8.EncodeRune(b, rr)
		b = b[:n]
		buf = append(buf, b...)
	}
	return buf
}
