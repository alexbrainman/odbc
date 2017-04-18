package odbc

import (
	"testing"

	"github.com/alexbrainman/odbc/api"
)

func Test_ErrorHandling_NewVariableWidthColumn(t *testing.T) {
	b := &BaseColumn{
		name: api.UTF16ToString(make([]uint16, 10)),
	}
	if c, err := NewVariableWidthColumn(b, api.SQL_UNKNOWN_TYPE, 1); err == nil {
		t.Errorf("Did not return error on unknown column type")
	} else {
		if c != nil {
			t.Errorf("Did not return nil Column when returning an error")
		}
	}
}

func Test_ErrorHandling_BindableColumn_Value(t *testing.T) {
	b := &BindableColumn{
		IsBound:         true,
		Len:             BufferLen(100),
		Size:            200,
		IsVariableWidth: false,
	}
	if v, err := b.Value(nil, 0); err == nil {
		t.Errorf("Did not return error on wrong column length")
	} else {
		if v != nil {
			t.Errorf("Did not return nil driver.Value when returning an error")
		}
	}
}

func Test_ToHandleAndType(t *testing.T) {
	h, ht, err := ToHandleAndType(nil)
	if err == nil {
		t.Errorf("Did not return error on bad handle")
	} else {
		if h != nil || ht != 0 {
			t.Errorf("Did not return zero values when returning an error")
		}
	}
}

func Test_ErrorHandling_Parameter_BindValue(t *testing.T) {
	var p Parameter
	var badValueType uint64
	if err := p.BindValue(nil, 0, badValueType); err == nil {
		t.Errorf("Did not return error on invalid value type")
	}
}

func Test_ErrorHandling_Stats_UpdateHandleCount(t *testing.T) {
	var s Stats
	if err := s.updateHandleCount(0, 0); err == nil {
		t.Errorf("Did not return error on unexpected handle type")
	}
}
