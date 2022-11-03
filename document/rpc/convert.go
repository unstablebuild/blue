package rpc

import (
	"github.com/ernestrc/blue/document"
	proto "github.com/ernestrc/blue/document/rpc/proto"
)

func makeProtoUpdates(m Marshaler, updates []document.Update) (
	ret []*proto.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range updates {
		// re-use make filter logic
		f := document.Filter{Field: document.Field{FieldPath: u.FieldPath, Value: u.Value}}
		pf := makeProtoFilter(m, slab, f)
		pu := &proto.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeProtoPreconditions(m Marshaler, preconds ...document.Precondition) (
	ret []*proto.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range preconds {
		f := document.Filter{Field: document.Field{FieldPath: u.FieldPath, Value: u.Value}}
		pf := makeProtoFilter(m, slab, f)
		pu := &proto.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeModelUpdates(m Marshaler, updates []*proto.UpdateDocumentRequest_Field) (
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

func makeModelPreconds(m Marshaler, preconds []*proto.UpdateDocumentRequest_Field) (
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

func makeModelFields(m Marshaler, fields []*proto.UpdateDocumentRequest_Field) (
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

func makeModelFilter(m Marshaler,
	slab map[string]interface{}, pf *proto.ListDocumentRequest_Filter,
) (document.Filter, error) {
	err := safeDecode(m, &slab, pf.Data)
	if err != nil {
		return document.Filter{}, err
	}

	return document.Filter{
		Field: document.Field{
			FieldPath: pf.FieldPath,
			Value:     slab["."],
		},
		Op: document.Op(pf.Operation),
	}, nil
}

func makeModelFilters(m Marshaler, filters []*proto.ListDocumentRequest_Filter) (
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
	m Marshaler,
	slab map[string]interface{}, f document.Filter,
) proto.ListDocumentRequest_Filter {
	// this is just atrick to be able to re-use encode functionality
	slab["."] = f.Value

	return proto.ListDocumentRequest_Filter{
		FieldPath: f.FieldPath,
		Data:      encode(m, slab, false),
		Operation: string(f.Op),
	}
}

func makeProtoFilters(m Marshaler, filters []document.Filter) (
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
