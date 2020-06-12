package datastore

import (
	"testing"
	"time"

	"github.com/ernestrc/blue/rpc"
	"github.com/stretchr/testify/assert"
)

func makeRPCStage(config *rpc.StageConfig) *rpc.Stage {
	now := stdTimeToProto(time.Now())
	return &rpc.Stage{
		Name:        str1,
		ProjectId:   str2,
		Config:      config,
		Description: str3,
		InputView:   str2,
		OutputView:  str1,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
}

func makeRPCPrototypeStage() *rpc.StageConfig {
	return &rpc.StageConfig{
		Config: &rpc.StageConfig_Prototype{
			Prototype: &rpc.PrototypeStage{
				Id:         str1,
				ConfigJson: str2,
			},
		},
	}
}

func assertEqualStages(t *testing.T, expectedEnt, outputEnt Entity) {
	output := outputEnt.(*Stage)
	expected := expectedEnt.(*Stage)

	expected.UpdatedAt = time.Time{}
	expected.CreatedAt = time.Time{}
	output.UpdatedAt = time.Time{}
	output.CreatedAt = time.Time{}

	assert.Equal(t, expected, output)
}

func TestStageProtoConversion(t *testing.T) {
	t.Run("prototype", func(t *testing.T) {
		rpcStage := makeRPCStage(makeRPCPrototypeStage())
		testEntityProtoConversion(t, new(Stage), rpcStage)
	})
}

func TestStagePersistence(t *testing.T) {
	var inputStage, outputStage Stage
	rpcStage := makeRPCStage(makeRPCPrototypeStage())
	inputStage.FromProto(rpcStage)
	testResourcePersistence(t, &inputStage, &outputStage, assertEqualStages)
}
