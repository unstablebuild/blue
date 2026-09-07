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

package docrpc

import (
	"strings"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docrpc/docpb"
)

// this is just a trick to be able to re-use encode functionality
const protoFieldKey = "X"

func makeProtoUpdates(m docmarshal.Marshaler, updates []document.Update) (
	ret []*docpb.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range updates {
		// re-use make filter logic
		f := document.Filter{Field: document.Field(u)}
		pf := makeProtoFilter(m, slab, f)
		pu := &docpb.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeProtoPreconditions(m docmarshal.Marshaler, preconds ...document.Precondition) (
	ret []*docpb.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range preconds {
		f := document.Filter{Field: document.Field(u)}
		pf := makeProtoFilter(m, slab, f)
		pu := &docpb.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeModelUpdates(m docmarshal.Marshaler, updates []*docpb.UpdateDocumentRequest_Field) (
	ret []document.Update, err error,
) {
	fields, err := makeModelFields(m, updates)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ret = append(ret, document.Update(field))
	}
	return
}

func makeModelPreconds(m docmarshal.Marshaler, preconds []*docpb.UpdateDocumentRequest_Field) (
	ret []document.Precondition, err error,
) {
	fields, err := makeModelFields(m, preconds)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ret = append(ret, document.Precondition(field))
	}
	return
}

func makeModelFields(m docmarshal.Marshaler, fields []*docpb.UpdateDocumentRequest_Field) (
	ret []document.Field, err error,
) {
	var slab map[string]interface{}
	var f document.Filter

	for _, u := range fields {
		// re-use make filter logic
		pf := docpb.ListDocumentRequest_Filter{
			FieldPath: u.FieldPath,
			Data:      u.Data,
		}
		f, err = makeModelFilter(m, slab, &pf)
		if err != nil {
			return
		}
		ret = append(ret, document.Field{
			FieldPath: f.FieldPath,
			Value:     f.Value,
		})
	}
	return
}

func makeModelFilter(m docmarshal.Marshaler,
	slab map[string]interface{}, pf *docpb.ListDocumentRequest_Filter,
) (document.Filter, error) {
	err := document.SafeDecode(m, &slab, pf.Data)
	if err != nil {
		return document.Filter{}, err
	}

	lowerCase := m.DefaultLowerCase()

	fieldPath := pf.FieldPath
	if lowerCase {
		fieldPath = make([]string, len(pf.FieldPath))
		for i, comp := range pf.FieldPath {
			fieldPath[i] = strings.ToLower(comp)
		}
	}

	return document.Filter{
		Field: document.Field{
			FieldPath: fieldPath,
			Value:     slab[protoFieldKey],
		},
		Op: document.Op(pf.Operation),
	}, nil
}

func makeModelFilters(m docmarshal.Marshaler, filters []*docpb.ListDocumentRequest_Filter) (
	ret []document.Filter, err error,
) {
	var slab map[string]interface{}
	for _, pf := range filters {
		var f document.Filter
		f, err = makeModelFilter(m, slab, pf)
		if err != nil {
			return
		}
		ret = append(ret, f)
	}
	return
}

func makeProtoFilter(
	m docmarshal.Marshaler,
	slab map[string]interface{}, f document.Filter,
) docpb.ListDocumentRequest_Filter {
	slab[protoFieldKey] = f.Value

	return docpb.ListDocumentRequest_Filter{
		FieldPath: f.FieldPath,
		Data:      document.Encode(m, slab, false),
		Operation: string(f.Op),
	}
}

func makeProtoFilters(m docmarshal.Marshaler, filters []document.Filter) (
	ret []*docpb.ListDocumentRequest_Filter, err error,
) {
	slab := make(map[string]interface{})
	for _, f := range filters {
		if len(f.FieldPath) == 0 {
			panic("invalid List filter: empty zero-valued FieldPath")
		}
		pf := new(docpb.ListDocumentRequest_Filter)
		*pf = makeProtoFilter(m, slab, f)

		ret = append(ret, pf)
	}
	return
}
