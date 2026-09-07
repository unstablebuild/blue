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

package cli

import (
	"context"
	"flag"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key string

// ContextWithOptions iterates over the options in fs and returns a context
// with the options defined as values.
//
// Options parsed are propagated to sub-command CLI via context.Context
// and can be retrieved by using OptionFromContext.
func ContextWithOptions(ctx context.Context, fs *FlagSet) context.Context {
	fs.Visit(func(f *flag.Flag) {
		getter := f.Value.(flag.Getter) // never panics, if Go >= 1
		ctx = context.WithValue(ctx, key(f.Name), getter.Get())
	})
	return ctx
}

// OptionFromContext attempts to retrieve the value
// of an flag named `name` from this context.
func OptionFromContext(ctx context.Context, name string) (
	v interface{}, ok bool,
) {
	v = ctx.Value(key(name))
	ok = v != nil
	return
}
