// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc_test

import (
	_ "code.google.com/p/odbc"
	"database/sql"
	"flag"
	"fmt"
	"testing"
	"time"
)

var (
	fox = flag.String("fox", "testdata", "directory where foxpro tables reside")
)

func TestFoxPro(t *testing.T) {
	conn := fmt.Sprintf("driver={Microsoft dBASE Driver (*.dbf)};driverid=277;dbq=%s;",
		*fox)

	db, err := sql.Open("odbc", conn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	type row struct {
		char       string
		num_2_0    sql.NullFloat64
		num_20_0   float64
		num_6_3    float32
		date       time.Time
		float_2_0  float32
		float_20_0 float64
		float_6_3  float32
		logical    bool
		memo       sql.NullString
	}

	var tests = []row{
		{
			char:       "123",
			num_2_0:    sql.NullFloat64{Float64: 1, Valid: true},
			num_20_0:   1232543,
			num_6_3:    12.73,
			date:       time.Date(2012, 5, 19, 0, 0, 0, 0, time.Local),
			float_2_0:  23,
			float_20_0: 12345678901234560,
			float_6_3:  12.345,
			logical:    true,
			memo:       sql.NullString{String: "Hello", Valid: true},
		},
		{
			char:       "abcdef",
			num_2_0:    sql.NullFloat64{Float64: 23, Valid: true},
			num_20_0:   4564568,
			num_6_3:    2,
			date:       time.Date(2012, 5, 20, 0, 0, 0, 0, time.Local),
			float_2_0:  1,
			float_20_0: 234,
			float_6_3:  0.123,
			logical:    false,
			memo:       sql.NullString{String: "", Valid: false},
		},
		{
			char:       "346546",
			num_2_0:    sql.NullFloat64{Float64: 4, Valid: true},
			num_20_0:   1234567890123456000,
			num_6_3:    99.99,
			date:       time.Date(2012, 5, 21, 0, 0, 0, 0, time.Local),
			float_2_0:  23,
			float_20_0: 457768,
			float_6_3:  99,
			logical:    true,
			memo:       sql.NullString{String: "World", Valid: true},
		},
		{
			char:       "asasds",
			num_2_0:    sql.NullFloat64{Float64: 0, Valid: false},
			num_20_0:   234456,
			num_6_3:    0.123,
			date:       time.Date(2012, 5, 22, 0, 0, 0, 0, time.Local),
			float_2_0:  65,
			float_20_0: 234,
			float_6_3:  1,
			logical:    false,
			memo:       sql.NullString{String: "12398y345 sdflkjdsfsd fds;lkdsfgl;sd", Valid: true},
		},
	}

	const query = `select id, 
		char, num_2_0, num_20_0, num_6_3, date,
		float_2_0, float_20_0, float_6_3, logical, memo
		from fldtest`
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int
		var r row
		err = rows.Scan(&id,
			&r.char, &r.num_2_0, &r.num_20_0, &r.num_6_3, &r.date,
			&r.float_2_0, &r.float_20_0, &r.float_6_3, &r.logical, &r.memo)
		if err != nil {
			t.Fatal(err)
		}

		if id < 0 || len(tests) < id {
			t.Errorf("unexpected row with id %d", id)
			continue
		}

		x := tests[id]
		if x.char != r.char {
			t.Errorf("row %d: char expected %v, but received %v", id, x.char, r.char)
		}
		if x.num_2_0 != r.num_2_0 {
			t.Errorf("row %d: num_2_0 expected %v, but received %v", id, x.num_2_0, r.num_2_0)
		}
		if x.num_20_0 != r.num_20_0 {
			t.Errorf("row %d: num_20_0 expected %v, but received %v", id, x.num_20_0, r.num_20_0)
		}
		if x.num_6_3 != r.num_6_3 {
			t.Errorf("row %d: num_6_3 expected %v, but received %v", id, x.num_6_3, r.num_6_3)
		}
		if x.date != r.date {
			t.Errorf("row %d: date expected %v, but received %v", id, x.date, r.date)
		}
		if x.float_2_0 != r.float_2_0 {
			t.Errorf("row %d: float_2_0 expected %v, but received %v", id, x.float_2_0, r.float_2_0)
		}
		if x.float_20_0 != r.float_20_0 {
			t.Errorf("row %d: float_20_0 expected %v, but received %v", id, x.float_20_0, r.float_20_0)
		}
		if x.float_6_3 != r.float_6_3 {
			t.Errorf("row %d: float_6_3 expected %v, but received %v", id, x.float_6_3, r.float_6_3)
		}
		if x.logical != r.logical {
			t.Errorf("row %d: logical expected %v, but received %v", id, x.logical, r.logical)
		}
		if x.memo != r.memo {
			t.Errorf("row %d: memo expected %v, but received %v", id, x.memo, r.memo)
		}
	}
	err = rows.Err()
	if err != nil {
		t.Fatal(err)
	}
}
