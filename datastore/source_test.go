package datastore

import (
	"testing"
	"time"

	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func makeGatewaySource(connector *rpc.Connector) *rpc.SourceConfig {
	return &rpc.SourceConfig{
		Config: &rpc.SourceConfig_Gateway{
			Gateway: &rpc.GatewaySource{
				Connector: connector,
			},
		},
	}
}

func makeRPCJdbcConnector() *rpc.Connector {
	return &rpc.Connector{
		Connector: &rpc.Connector_Jdbc{
			Jdbc: &rpc.JdbcConnector{
				Url:            str1,
				Driver:         str2,
				User:           str3,
				Password:       str1,
				IgnoreUserMode: trueBool,
			},
		},
	}
}

func makeRPCKafkaConnector() *rpc.Connector {
	return &rpc.Connector{
		Connector: &rpc.Connector_Kafka{
			Kafka: &rpc.KafkaConnector{
				KeyDeserializer:     str1,
				ValueDeserializer:   str2,
				IgnoreParsingErrors: trueBool,
				ConsumerConfig:      &struct1,
			},
		},
	}
}

func makeRPCZoqlConnector() *rpc.Connector {
	return &rpc.Connector{
		Connector: &rpc.Connector_Zoql{
			Zoql: &rpc.ZOQLConnector{
				Host:      str1,
				Password:  str1,
				User:      str2,
				FetchSize: num1,
			},
		},
	}
}

func makeRPCPrestoConnector() *rpc.Connector {
	return &rpc.Connector{
		Connector: &rpc.Connector_Presto{
			Presto: &rpc.PrestoConnector{
				Host: str1,
				Port: num1,
			},
		},
	}
}

func makeRPCElasticConnector() *rpc.Connector {
	return &rpc.Connector{
		Connector: &rpc.Connector_Elastic{
			Elastic: &rpc.ElasticSearchConnector{
				Host: str1,
				Port: num1,
			},
		},
	}
}

func makeRPCSonicConnector() *rpc.Connector {
	return &rpc.Connector{
		Connector: &rpc.Connector_Sonic{
			Sonic: &rpc.SonicConnector{
				Host:   str1,
				Port:   num1,
				Config: &struct1,
			},
		},
	}
}
func makeRPCSyntheticConnector() *rpc.Connector {
	return &rpc.Connector{
		Connector: &rpc.Connector_Synthetic{
			Synthetic: &rpc.SyntheticConnector{
				Seed:            num1,
				Size:            num1,
				ProgressDelayMs: num1,
				Indexed:         trueBool,
				Schema:          &struct1,
			},
		},
	}
}
func makeRPCSource(config *rpc.SourceConfig) *rpc.Source {
	now := stdTimeToProto(time.Now())
	return &rpc.Source{
		Name:        str1,
		ProjectId:   str2,
		Description: str3,
		Config:      config,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
}

func testEntityProtoConversion(t *testing.T, in Entity, p proto.Message) {
	in.FromProto(p)
	out := in.ToProto()
	in.FromProto(out)
	out = in.ToProto()

	// m := jsonpb.Marshaler{}
	// instr, err := m.MarshalToString(p)
	// require.NoError(t, err)
	// outstr, err := m.MarshalToString(out)
	// require.NoError(t, err)

	// assert.Equal(t, instr, outstr)
	assert.Equal(t, p, out)
}

func TestSourceProtoConversion(t *testing.T) {
	t.Run("gw jdbc", func(t *testing.T) {
		rpcSource := makeRPCSource(makeGatewaySource(makeRPCJdbcConnector()))
		testEntityProtoConversion(t, new(Source), rpcSource)
	})
	t.Run("gw kafka", func(t *testing.T) {
		rpcSource := makeRPCSource(makeGatewaySource(makeRPCKafkaConnector()))
		testEntityProtoConversion(t, new(Source), rpcSource)
	})
	t.Run("gw zoql", func(t *testing.T) {
		rpcSource := makeRPCSource(makeGatewaySource(makeRPCZoqlConnector()))
		testEntityProtoConversion(t, new(Source), rpcSource)
	})
	t.Run("gw elastic", func(t *testing.T) {
		rpcSource := makeRPCSource(makeGatewaySource(makeRPCElasticConnector()))
		testEntityProtoConversion(t, new(Source), rpcSource)
	})
	t.Run("gw synthetic", func(t *testing.T) {
		rpcSource := makeRPCSource(makeGatewaySource(makeRPCSyntheticConnector()))
		testEntityProtoConversion(t, new(Source), rpcSource)
	})
	t.Run("gw sonic", func(t *testing.T) {
		rpcSource := makeRPCSource(makeGatewaySource(makeRPCSonicConnector()))
		testEntityProtoConversion(t, new(Source), rpcSource)
	})
	t.Run("gw presto", func(t *testing.T) {
		rpcSource := makeRPCSource(makeGatewaySource(makeRPCPrestoConnector()))
		testEntityProtoConversion(t, new(Source), rpcSource)
	})
}

func assertEqualSources(t *testing.T, expectedEnt, outputEnt Entity) {
	expected := expectedEnt.(*Source)
	output := outputEnt.(*Source)

	expected.UpdatedAt = time.Time{}
	expected.CreatedAt = time.Time{}
	expected.Config.Gateway.Connector.Sonic.Config = make(map[string]interface{})
	expected.Config.Gateway.Connector.Kafka.ConsumerConfig = make(map[string]interface{})

	output.UpdatedAt = time.Time{}
	output.CreatedAt = time.Time{}

	assert.Equal(t, expected, output)
}

func TestSourcePersistence(t *testing.T) {
	var inputSource, outputSource Source
	rpcSource := makeRPCSource(makeGatewaySource(makeRPCSyntheticConnector()))
	inputSource.FromProto(rpcSource)

	testResourcePersistence(t, &inputSource, &outputSource, assertEqualSources)
}
