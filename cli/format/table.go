package format

import (
	"fmt"
	"io"
	"reflect"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/olekukonko/tablewriter"
)

// Table returns an IteratorFormatter that formats elements
// into a table of the given fields.
func Table(fields []string) IteratorFormatter {
	set := make(map[string]int)
	for i, f := range fields {
		set[f] = i
	}
	return tableFormatter{fields: fields, set: set}
}

type tableFormatter struct {
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

func (f tableFormatter) Format(w io.Writer, it iterator.Iterator) error {
	table := tablewriter.NewWriter(w)
	table.SetHeader(f.fields)

	for {
		t, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		v, ok := isEncodeable(t)
		if !ok {
			err = fmt.Errorf("iterator returned value that cannot be formatted: %v", t)
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
