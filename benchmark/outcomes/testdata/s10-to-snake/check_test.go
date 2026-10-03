package task

import (
	"testing"
)

func TestToSnake(t *testing.T) {
	for in, want := range map[string]string{"HTTPServer": "http_server", "userID": "user_id", "simple": "simple", "CamelCase": "camel_case", "parseJSONBody": "parse_json_body", "v2Api": "v2_api"} {
		if got := ToSnake(in); got != want {
			t.Errorf("ToSnake(%q)=%q want %q", in, got, want)
		}
	}
}
