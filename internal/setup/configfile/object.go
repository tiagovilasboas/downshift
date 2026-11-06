// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package configfile holds the primitives setup uses to edit a harness's
// config files without disturbing the rest: an order-keeping JSON object, a
// member list that keeps each value's original text, a unified preview of a
// change, an atomic write, and the marker that tells a file setup owns.
// It knows no harness, target or plan; those stay with setup.
package configfile

// Object is a JSON object that keeps key order, so a merge into a developer's
// settings file does not reshuffle keys they never touched. Values are *Object,
// []any, string, json.Number, bool or nil.
type Object struct {
	keys []string
	vals map[string]any
}

// NewObject returns an empty object.
func NewObject() *Object { return &Object{vals: map[string]any{}} }

// Get returns the value at k and whether k is present.
func (o *Object) Get(k string) (any, bool) { v, ok := o.vals[k]; return v, ok }

// Value returns the value at k, or nil when k is absent.
func (o *Object) Value(k string) any { return o.vals[k] }

// Keys returns a copy of the keys in file order.
func (o *Object) Keys() []string { return append([]string(nil), o.keys...) }

// Len is the number of keys.
func (o *Object) Len() int { return len(o.keys) }

// Set stores v at k; a new key goes last, an existing one keeps its place.
func (o *Object) Set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

// Del removes k, if present.
func (o *Object) Del(k string) {
	if _, ok := o.vals[k]; !ok {
		return
	}
	delete(o.vals, k)
	for i, key := range o.keys {
		if key == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// AsObject reports whether v is a JSON object read by ParseObject.
func AsObject(v any) (*Object, bool) { o, ok := v.(*Object); return o, ok }
