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

package doctest

import (
	"testing"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
)

func TestInMemoryService(t *testing.T) {
	tsuite := []struct {
		encoding  string
		marshaler docmarshal.Marshaler
	}{
		{"bson", docbson.Marshaler()},
		{"json", docjson.Marshaler()},
		{"toml", doctoml.Marshaler()},
		// NOTE: yaml passes all tests except the ones with encoding of
		// numerical values. It should never be used as a storage format anyway.
		// {"yaml", yaml.Marshaler()},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.encoding, func(t *testing.T) {
			TestDocumentService(t, func(t *testing.T) document.Service {
				return document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
			})
			// NOTE: time preconditions don't quite work in json
			// or toml due to lossy time marshalling
			if tcase.encoding == "json" || tcase.encoding == "toml" {
				return
			}
			TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
				return document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
			})
		})
	}
}
