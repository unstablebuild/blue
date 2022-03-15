package datastore

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProtoTimeToStd converts timestamp.Timestamp (proto) into time.Time
func ProtoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}

// StdTimeToProto converts time.Time into timestamp.Timestamp (proto).
func StdTimeToProto(ts time.Time) timestamppb.Timestamp {
	seconds := ts.Unix()
	nanos := ts.Nanosecond()
	return timestamppb.Timestamp{Seconds: seconds, Nanos: int32(nanos)}
}

func protoValueToIface(in *structpb.Value) interface{} {
	switch in.GetKind().(type) {
	case *structpb.Value_NullValue:
		return nil
	case *structpb.Value_NumberValue:
		return in.GetNumberValue()
	case *structpb.Value_StringValue:
		return in.GetStringValue()
	case *structpb.Value_BoolValue:
		return in.GetBoolValue()
	case *structpb.Value_StructValue:
		fields := in.GetStructValue().GetFields()
		return protoMapToStdMap(fields)
	case *structpb.Value_ListValue:
		var res []interface{}
		values := in.GetListValue().Values
		for _, v := range values {
			res = append(res, protoValueToIface(v))
		}
		return res
	}

	panic("unknown structpb.Value")
}

func ifaceToProtoValue(in interface{}) (out *structpb.Value) {
	out = new(structpb.Value)
	switch in.(type) {
	// values are always converted first using protoValueToIface
	// so the scope of this type switch is limited to the values
	// returned by that function
	case float64:
		out.Kind = &structpb.Value_NumberValue{NumberValue: in.(float64)}
	case string:
		out.Kind = &structpb.Value_StringValue{StringValue: in.(string)}
	case nil:
		out.Kind = &structpb.Value_NullValue{}
	case bool:
		out.Kind = &structpb.Value_BoolValue{BoolValue: in.(bool)}
	case map[string]interface{}:
		out.Kind = &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: stdMapToProtoMap(in.(map[string]interface{})),
			},
		}
	case []interface{}:
		var res []*structpb.Value
		values := in.([]interface{})
		for _, v := range values {
			res = append(res, ifaceToProtoValue(v))
		}
		out.Kind = &structpb.Value_ListValue{
			ListValue: &structpb.ListValue{
				Values: res,
			},
		}
	default:
		panic("unknown interface{} type for structpb.Value")
	}

	return out
}

func protoMapToStdMap(
	in map[string]*structpb.Value,
) (out map[string]interface{}) {
	out = make(map[string]interface{})

	for k, v := range in {
		out[k] = protoValueToIface(v)
	}

	return
}

func stdMapToProtoMap(
	in map[string]interface{},
) (out map[string]*structpb.Value) {
	out = make(map[string]*structpb.Value)

	for k, v := range in {
		out[k] = ifaceToProtoValue(v)
	}

	return
}

// ParseDocumentID extracts a projectID and a resource name from id.
func ParseDocumentID(id string) (projectID, name string) {
	a := strings.Split(string(id), ".")
	if len(a) != 2 {
		panic(fmt.Sprintf("document ID: corrupt: %+v", a))
	}

	projectID64, err := base64.StdEncoding.DecodeString(a[0])
	if err != nil {
		panic(fmt.Sprintf("document ID: decode projectID error: %s", err))
	}

	nameID64, err := base64.StdEncoding.DecodeString(a[1])
	if err != nil {
		panic(fmt.Sprintf("document ID: decode name error: %s", err))
	}

	projectID = string(projectID64)
	name = string(nameID64)

	return
}

// MakeDocumentID generates an encoded document ID for the given resource
// name and projectID.
func MakeDocumentID(projectID, name string) string {
	name = base64.StdEncoding.EncodeToString([]byte(name))
	projectID = base64.StdEncoding.EncodeToString([]byte(projectID))
	return fmt.Sprintf("%s.%s", projectID, name)
}
