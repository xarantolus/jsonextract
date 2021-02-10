package jsonextract

import (
	"reflect"
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		arg  string
		want []string
	}{
		{
			`{"a": "b"}`,
			[]string{`{"a": "b"}`},
		},
		{
			"[1, 3, 55]",
			[]string{"[1, 3, 55]"},
		},
		{
			"[1, 3, 55, ]",
			nil,
		},
		{
			"askdflaksmvalsd",
			nil,
		},
		{
			`"json encoded text\nNew line"`,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			if gotExtracted := String(tt.arg); !reflect.DeepEqual(gotExtracted, tt.want) {
				t.Errorf("String() = %v, want %v", gotExtracted, tt.want)
			}
		})
	}
}
