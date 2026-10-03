package task

import (
	"errors"
	"strings"
)

func ParseCSVLine(line string) ([]string, error) {
	var fields []string
	var cur strings.Builder
	inQuotes := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQuotes && c == '"' && i+1 < len(line) && line[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			inQuotes = !inQuotes
		case c == ',' && !inQuotes:
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if inQuotes {
		return nil, errors.New("unterminated quote")
	}
	return append(fields, cur.String()), nil
}
