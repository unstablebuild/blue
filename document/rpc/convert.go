package rpc

import (
	"strings"

	"github.com/unstablebuild/blue/document"
	proto "github.com/unstablebuild/blue/document/rpc/proto"
	"github.com/unstablebuild/blue/encoding"
)

// this is just a trick to be able to re-use encode functionality
const protoFieldKey = "X"

func makeProtoUpdates(m encoding.Marshaler, updates []document.Update) (
	ret []*proto.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range updates {
		// re-use make filter logic
		f := document.Filter{Field: document.Field(u)}
		pf := makeProtoFilter(m, slab, f)
		pu := &proto.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeProtoPreconditions(m encoding.Marshaler, preconds ...document.Precondition) (
	ret []*proto.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range preconds {
		f := document.Filter{Field: document.Field(u)}
		pf := makeProtoFilter(m, slab, f)
		pu := &proto.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeModelUpdates(m encoding.Marshaler, updates []*proto.UpdateDocumentRequest_Field) (
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

func makeModelPreconds(m encoding.Marshaler, preconds []*proto.UpdateDocumentRequest_Field) (
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

func makeModelFields(m encoding.Marshaler, fields []*proto.UpdateDocumentRequest_Field) (
	ret []document.Field, err error,
) {
	var slab map[string]interface{}
	var f document.Filter

	for _, u := range fields {
		// re-use make filter logic
		pf := proto.ListDocumentRequest_Filter{
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

func makeModelFilter(m encoding.Marshaler,
	slab map[string]interface{}, pf *proto.ListDocumentRequest_Filter,
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

func makeModelFilters(m encoding.Marshaler, filters []*proto.ListDocumentRequest_Filter) (
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
	m encoding.Marshaler,
	slab map[string]interface{}, f document.Filter,
) proto.ListDocumentRequest_Filter {
	slab[protoFieldKey] = f.Value

	return proto.ListDocumentRequest_Filter{
		FieldPath: f.FieldPath,
		Data:      document.Encode(m, slab, false),
		Operation: string(f.Op),
	}
}

func makeProtoFilters(m encoding.Marshaler, filters []document.Filter) (
	ret []*proto.ListDocumentRequest_Filter, err error,
) {
	slab := make(map[string]interface{})
	for _, f := range filters {
		if len(f.Field.FieldPath) == 0 {
			panic("invalid List filter: empty zero-valued FieldPath")
		}
		pf := new(proto.ListDocumentRequest_Filter)
		*pf = makeProtoFilter(m, slab, f)

		ret = append(ret, pf)
	}
	return
}
