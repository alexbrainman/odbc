// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

func TestAccessMemo(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestAccessMemo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	dbfilename := filepath.Join(tmpdir, "db.mdb")
	createAccessDB(t, dbfilename)

	db, err := sql.Open("odbc", fmt.Sprintf("DRIVER={Microsoft Access Driver (*.mdb)};DBQ=%s;", dbfilename))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("create table mytable (m memo)")
	if err != nil {
		t.Fatal(err)
	}
	for s := ""; len(s) < 1000; s += "0123456789" {
		_, err = db.Exec("insert into mytable (m) values (?)", s)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func createAccessDB(t *testing.T, dbfilename string) {
	err := ole.CoInitialize(0)
	if err != nil {
		t.Fatal(err)
	}
	defer ole.CoUninitialize()

	unk, err := oleutil.CreateObject("adox.catalog")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		t.Fatal(err)
	}
	_, err = oleutil.CallMethod(cat, "create", fmt.Sprintf("provider=microsoft.jet.oledb.4.0;data source=%s;", dbfilename))
	if err != nil {
		t.Fatal(err)
	}
}
