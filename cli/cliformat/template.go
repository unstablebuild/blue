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

func (f templateFormatter[T]) Format(
	ctx context.Context, w io.Writer, it iterator.Iterator[T],
) error {
	for {
		t, ok := it.Next(ctx)
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
		_, _ = fmt.Fprint(w, "\n")
	}
	return nil
}
