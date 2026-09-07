// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cliformat

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"github.com/olekukonko/tablewriter"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

// Table returns an IteratorFormatter that formats elements
// into a table of the given fields.
func Table[T any](fields []string) IteratorFormatter[T] {
	set := make(map[string]int)
	for i, f := range fields {
		set[f] = i
	}
	return tableFormatter[T]{fields: fields, set: set}
}

type tableFormatter[T any] struct {
	fields []string
	set    map[string]int
}

func isEncodeable(t any) (reflect.Value, bool) {
	v := reflect.ValueOf(t)
	t, err := document.DerefCreateValue(v)
	if err != nil {
		return reflect.Value{}, false
	}
	v = reflect.ValueOf(t)
	switch v.Kind() {
	case reflect.Map:
		return v, !v.IsNil()
	case reflect.Struct:
		return v, true
	default:
		return reflect.Value{}, false
	}
}

func (f tableFormatter[T]) Format(
	ctx context.Context, w io.Writer, it iterator.Iterator[T],
) error {
	table := tablewriter.NewWriter(w)
	table.Header(f.fields)

	for {
		t, ok := it.Next(ctx)
		if !ok {
			if err := it.Err(); err != nil {
				return err
			}
			break
		}
		v, ok := isEncodeable(t)
		if !ok {
			err := fmt.Errorf("iterator returned value that cannot be formatted: %v", t)
			return err
		}
		row := make([]string, len(f.set))
		for ff, pos := range f.set {
			var field reflect.Value
			if v.Kind() == reflect.Map {
				field = v.MapIndex(reflect.ValueOf(ff))
			} else {
				field = v.FieldByName(ff)
			}
			if field.CanInterface() {
				row[pos] = fmt.Sprintf("%v", field.Interface())
			}
		}
		_ = table.Append(row)
	}
	_ = table.Render()
	return nil
}
