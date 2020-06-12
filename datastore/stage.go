package datastore

import (
	"fmt"
	"time"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
)

// Stage represents a more idiomatic version of rpc.Stage.
// It satisfies datastore.Entity so it can be converted from/to rpc.Stage.
type Stage struct {
	ID          string
	Name        string
	ProjectID   string
	Description string

	InputView  string
	OutputView string

	Type   StageConfigType
	Config StageConfigUnion

	UpdatedAt time.Time `firestore:",serverTimestamp"`
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

// StageConfigUnion is a type that encapsulates one of
// the many stage configurations. See SourceConfigUnion for more information.
type StageConfigUnion struct {
	Prototype StagePrototype
}

// StageConfigType of a StageConfigUnion.
type StageConfigType uint8

const (
	// StageConfigTypePrototype is the StageConfigType of rpc.PrototypeStage
	StageConfigTypePrototype StageConfigType = iota
)

// StagePrototype is a rpc.StagePrototype which allows quick prototyping of
// new stages. ID maps to one of the types of prototypes and ConfigJSON is
// the JSON config that's passed to the prototype.
type StagePrototype struct {
	ID         string
	ConfigJSON string
}

// FromProto panics if msg is not a *rpc.Stage
func (s *Stage) FromProto(msg proto.Message) {
	stage, ok := msg.(*rpc.Stage)
	if !ok {
		panic(ErrInvalidProtoMessage)
	}

	name := stage.GetName()

	s.ProjectID = stage.GetProjectId()
	s.ID = MakeDocumentID(s.ProjectID, name)
	s.Name = name
	s.InputView = stage.GetInputView()
	s.OutputView = stage.GetOutputView()
	s.Description = stage.GetDescription()
	s.CreatedAt = protoTimeToStd(stage.GetCreatedAt())
	s.UpdatedAt = protoTimeToStd(stage.GetUpdatedAt())

	switch stage.Config.Config.(type) {
	case *rpc.StageConfig_Prototype:
		var prototype StagePrototype
		prototype.FromProto(stage.GetConfig().GetPrototype())

		s.Config.Prototype = prototype
		s.Type = StageConfigTypePrototype
	default:
		panic(fmt.Sprintf("unexpected type: %+v", stage.Config.Config))
	}
}

// ToProto converts s into *rpc.Stage
func (s *Stage) ToProto() proto.Message {
	config := &rpc.StageConfig{}
	switch s.Type {
	case StageConfigTypePrototype:
		protoGw := s.Config.Prototype.ToProto()
		config.Config = &rpc.StageConfig_Prototype{
			Prototype: protoGw.(*rpc.PrototypeStage),
		}
	default:
		panic(fmt.Sprintf("unexpected config type: %d", s.Type))
	}

	createdAt := stdTimeToProto(s.CreatedAt)
	updatedAt := stdTimeToProto(s.UpdatedAt)

	return &rpc.Stage{
		Name:        s.Name,
		ProjectId:   s.ProjectID,
		Description: s.Description,
		InputView:   s.InputView,
		OutputView:  s.OutputView,
		Config:      config,
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
	}
}

// ToProtoResource converts s into a *rpc.Resource
func (s *Stage) ToProtoResource() *rpc.Resource {
	return &rpc.Resource{
		Resource: &rpc.Resource_Stage{
			Stage: s.ToProto().(*rpc.Stage),
		},
	}
}

// FromProto panics if msg is not *rpc.GatewaySource
func (s *StagePrototype) FromProto(msg proto.Message) {
	prototype, ok := msg.(*rpc.PrototypeStage)
	if !ok {
		panic(ErrInvalidProtoMessage)
	}
	s.ID = prototype.GetId()
	s.ConfigJSON = prototype.GetConfigJson()
}

// ToProto returns s as *rpc.GatewaySource
func (s *StagePrototype) ToProto() proto.Message {
	return &rpc.PrototypeStage{
		Id:         s.ID,
		ConfigJson: s.ConfigJSON,
	}
}

// WithUpdateStageConfig returns an updated slice of updates
// which updates a Stage config.
func WithUpdateStageConfig(
	in []document.Update, cfg *rpc.StageConfig,
) []document.Update {
	switch cfg.Config.(type) {
	case *rpc.StageConfig_Prototype:
		var prototype StagePrototype
		prototype.FromProto(cfg.GetPrototype())
		return append(in,
			document.Update{FieldPath: []string{"Type"}, Value: SourceKindGateway},
			document.Update{FieldPath: []string{"Config", "Prototype"}, Value: prototype},
		)
	default:
		panic(fmt.Sprintf("unexpected type: %+v", cfg.Config))
	}
}

// WithUpdateInputView returns an updated slice of updates
// which updates a document field `desc`.
func WithUpdateInputView(in []document.Update, view string) []document.Update {
	return append(in, document.Update{FieldPath: []string{"InputView"}, Value: view})
}

// WithUpdateOutputView returns an updated slice of updates
// which updates a document field `desc`.
func WithUpdateOutputView(in []document.Update, view string) []document.Update {
	return append(in, document.Update{FieldPath: []string{"OutputView"}, Value: view})
}
