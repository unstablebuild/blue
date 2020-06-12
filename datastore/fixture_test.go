package datastore

import structpb "github.com/golang/protobuf/ptypes/struct"

var (
	str1     = "jdbc://root@127.0.0.1:3336"
	str2     = "mysql"
	str3     = "root"
	num1     = int32(1234)
	trueBool = true
	fields1  = make(map[string]*structpb.Value)
	struct1  = structpb.Struct{
		Fields: fields1,
	}
)

func init() {
	fields1["null"] = &structpb.Value{
		Kind: &structpb.Value_NullValue{},
	}
	fields1["number"] = &structpb.Value{
		Kind: &structpb.Value_NumberValue{NumberValue: 123.453},
	}
	fields1["string"] = &structpb.Value{
		Kind: &structpb.Value_StringValue{StringValue: ""},
	}
	fields1["bool"] = &structpb.Value{
		Kind: &structpb.Value_BoolValue{BoolValue: true},
	}
	// case *structpb.Value_ListValue:
	fields1["list"] = &structpb.Value{
		Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{
			Values: []*structpb.Value{
				fields1["bool"], fields1["string"],
				fields1["null"], fields1["number"],
			},
		}},
	}

	inCopy := make(map[string]*structpb.Value)
	for k, v := range fields1 {
		inCopy[k] = v
	}
	// case *structpb.Value_StructValue:
	fields1["struct"] = &structpb.Value{
		Kind: &structpb.Value_StructValue{StructValue: &structpb.Struct{
			Fields: inCopy,
		}},
	}
}
