// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package operatorexport

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
)

// The allowlist is the sensor for issue #104: an outside operator must be able
// to paste the export with no private string ever in it. Check walks the JSON
// against the schema of the Export struct, derived by reflection from its json
// tags, and refuses:
//   - an object key the schema does not have, whatever its value (a map keyed
//     by a repo, a model or a host is refused even with numeric values);
//   - a value of the wrong JSON type for its field;
//   - a string that its own field does not allow (stringRules).
//
// The schema itself is closed: a field whose Go type is a map, an interface or
// a string with no rule makes schema construction fail, so a leak cannot be
// added to Export without a deliberate edit here.

// stringRules are the only fields that may hold a string, each with the check
// its value must pass. Every other leaf is a count or a token total.
var stringRules = map[string]func(string) bool{
	// An RFC3339 timestamp; carries no identity. It is on the root only,
	// because only Export has the field.
	"generated_at": func(v string) bool {
		_, err := time.Parse(time.RFC3339, v)
		return err == nil
	},
	// The fixed honesty line.
	"note": func(v string) bool { return v == Note },
	// A known adapter name, or the token aggregation's unknown sentinel.
	"harness": func(v string) bool { return knownHarnessValue(v) },
}

// knownHarnessValue reports whether v may appear as a harness name.
func knownHarnessValue(v string) bool {
	if v == accounting.UnknownHarness {
		return true
	}
	for _, h := range knownHarnessNames() {
		if h == v {
			return true
		}
	}
	return false
}

type nodeKind int

const (
	kindNumber nodeKind = iota
	kindString
	kindObject
	kindArray
)

// schemaNode is one position in the export's JSON shape.
type schemaNode struct {
	kind   nodeKind
	fields map[string]*schemaNode // kindObject
	elem   *schemaNode            // kindArray
	rule   func(string) bool      // kindString
}

var (
	exportSchemaOnce sync.Once
	exportSchema     *schemaNode
	exportSchemaErr  error
)

func schema() (*schemaNode, error) {
	exportSchemaOnce.Do(func() {
		exportSchema, exportSchemaErr = schemaOf(reflect.TypeOf(Export{}), "")
	})
	return exportSchema, exportSchemaErr
}

// schemaOf derives the JSON shape of t. name is the json key t sits under,
// used to bind a string to its rule and to name the offending field.
func schemaOf(t reflect.Type, name string) (*schemaNode, error) {
	switch t.Kind() {
	case reflect.Int, reflect.Int64:
		return &schemaNode{kind: kindNumber}, nil
	case reflect.String:
		rule, ok := stringRules[name]
		if !ok {
			return nil, fmt.Errorf("field %q is a string with no allowlist rule", name)
		}
		return &schemaNode{kind: kindString, rule: rule}, nil
	case reflect.Slice:
		elem, err := schemaOf(t.Elem(), name)
		if err != nil {
			return nil, err
		}
		return &schemaNode{kind: kindArray, elem: elem}, nil
	case reflect.Struct:
		node := &schemaNode{kind: kindObject, fields: map[string]*schemaNode{}}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			key := strings.Split(f.Tag.Get("json"), ",")[0]
			if key == "" || key == "-" {
				return nil, fmt.Errorf("field %s.%s has no json name", t.Name(), f.Name)
			}
			child, err := schemaOf(f.Type, key)
			if err != nil {
				return nil, err
			}
			node.fields[key] = child
		}
		return node, nil
	}
	return nil, fmt.Errorf("field %q has type %s, which the export does not allow", name, t)
}

// Check renders an export and runs CheckJSON on it.
func Check(e Export) error {
	raw, err := JSON(e)
	if err != nil {
		return fmt.Errorf("render export: %w", err)
	}
	return CheckJSON([]byte(raw))
}

// CheckJSON checks raw JSON bytes against the export schema, so a test can
// feed a deliberately leaky fixture that never went through Build. A nil
// return means every key and every string in the export is accounted for.
func CheckJSON(raw []byte) error {
	root, err := schema()
	if err != nil {
		return fmt.Errorf("export schema: %w", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return fmt.Errorf("parse export json: %w", err)
	}
	return walk("", tree, root)
}

// walk checks node against want. field is the key node sits under.
func walk(field string, node any, want *schemaNode) error {
	switch want.kind {
	case kindObject:
		obj, ok := node.(map[string]any)
		if !ok {
			return fmt.Errorf("field %q must be an object", field)
		}
		for _, key := range sortedKeys(obj) {
			child, ok := want.fields[key]
			if !ok {
				return fmt.Errorf("field %q is not in the export schema", key)
			}
			if err := walk(key, obj[key], child); err != nil {
				return err
			}
		}
	case kindArray:
		items, ok := node.([]any)
		if !ok {
			return fmt.Errorf("field %q must be an array", field)
		}
		for _, item := range items {
			if err := walk(field, item, want.elem); err != nil {
				return err
			}
		}
	case kindString:
		s, ok := node.(string)
		if !ok {
			return fmt.Errorf("field %q must be a string", field)
		}
		if !want.rule(s) {
			return fmt.Errorf("field %q has value %q, which is not on the value allowlist", field, s)
		}
	case kindNumber:
		if _, ok := node.(float64); !ok {
			return fmt.Errorf("field %q must be a number, got %T", field, node)
		}
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
