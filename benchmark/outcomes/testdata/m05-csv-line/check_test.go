package task

import (
	"reflect"
	"testing"
)

func TestParseCSVLine(t *testing.T) {
	cases := map[string][]string{
		`a,b,c`:          {"a", "b", "c"},
		`"x, y",z`:       {"x, y", "z"},
		`"say ""hi""",2`: {`say "hi"`, "2"},
		`,,`:             {"", "", ""},
		``:               {""},
		`"",end`:         {"", "end"},
	}
	for in, want := range cases {
		got, err := ParseCSVLine(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q => %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseCSVLine(`"open,field`); err == nil {
		t.Error("expected unterminated quote error")
	}
}
