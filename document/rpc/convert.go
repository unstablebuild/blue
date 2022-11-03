package rpc

import (
	"github.com/ernestrc/blue/document"
	proto "github.com/ernestrc/blue/document/rpc/proto"
)

func makeProtoUpdates(updates []document.Update) (
	ret []*proto.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range updates {
		// re-use make filter logic
		f := document.Filter{Field: document.Field{FieldPath: u.FieldPath, Value: u.Value}}
		pf := makeProtoFilter(slab, f)
		pu := &proto.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeProtoPreconditions(preconds ...document.Precondition) (
	ret []*proto.UpdateDocumentRequest_Field,
) {
	slab := make(map[string]interface{})
	for _, u := range preconds {
		f := document.Filter{Field: document.Field{FieldPath: u.FieldPath, Value: u.Value}}
		pf := makeProtoFilter(slab, f)
		pu := &proto.UpdateDocumentRequest_Field{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeModelUpdates(updates []*proto.UpdateDocumentRequest_Field) (
	ret []document.Update, err error,
) {
	fields, err := makeModelFields(updates)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ret = append(ret, document.Update(field))
	}
	return
}

func makeModelPreconds(preconds []*proto.UpdateDocumentRequest_Field) (
	ret []document.Precondition, err error,
) {
	fields, err := makeModelFields(preconds)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ret = append(ret, document.Precondition(field))
	}
	return
}

func makeModelFields(fields []*proto.UpdateDocumentRequest_Field) (
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
		f, err = makeModelFilter(slab, &pf)
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
