package format

import (
	"fmt"
	"io"
	"reflect"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/olekukonko/tablewriter"
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

func isEncodeable(t interface{}) (reflect.Value, bool) {
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

func (f tableFormatter[T]) Format(w io.Writer, it iterator.Iterator[T]) error {
	table := tablewriter.NewWriter(w)
	table.SetHeader(f.fields)

	for {
		t, ok := it.Next()
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
		table.Append(row)
	}
	table.Render()
	return nil
}
