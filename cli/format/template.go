package format

import (
	"fmt"
	"io"
	"text/template"

	"github.com/unstablebuild/blue/iterator"
)

// Template returns an IteratorFormatter that formats elements
// according to the given Go text/template template.
// See https://pkg.go.dev/text/template for more details.
func Template[T any](tmpl string) (IteratorFormatter[T], error) {
	t, err := template.New("temp").Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("invalid args: not a valid Go template: "+
			"%s. See https://pkg.go.dev/text/template", tmpl)
	}
	return templateFormatter[T]{tmpl: t}, nil
}

type templateFormatter[T any] struct {
	tmpl *template.Template
}

func (f templateFormatter[T]) Format(w io.Writer, it iterator.Iterator[T]) error {
	for {
		t, ok := it.Next()
		if !ok {
			if err := it.Err(); err != nil {
				return err
			}
			break
		}
		err := f.tmpl.Execute(w, t)
		if err != nil {
			return err
		}
		fmt.Fprint(w, "\n")
	}
	return nil
}
