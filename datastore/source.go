package datastore

import (
	"fmt"
	"time"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
	structpb "github.com/golang/protobuf/ptypes/struct"
)

// Source represents a more idiomatic version of rpc.Source.
// It satisfies datastore.Entity so it can be converted from/to rpc.Source.
type Source struct {
	ID          string
	Name        string
	ProjectID   string
	Description string
	Kind        SourceKind
	Config      SourceConfigUnion
	Verify      bool

	UpdatedAt time.Time `firestore:",serverTimestamp"`
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

// SourceConfigUnion is a type that encapsulates one of the many source configurations.
// Marshaler/unmarshalers tipically reject interface types
// that are not the empty interface. Defining a custom alias of
// the empty interface both allows us to keep the marshalers/unmarshalers
// happy, and provide some form type safety when dealing with these types.
type SourceConfigUnion struct {
	Gateway SourceGateway
	File    SourceFile
}

type SourceKind uint8

const (
	// SourceKindGateway is the SourceKind
	SourceKindGateway SourceKind = iota
	SourceKindFile
)

type SourceConfig interface {
	Kind() SourceKind
	ToProto() proto.Message
	FromProto(proto.Message)
}

// FromProto panics if msg is not a *rpc.Source
func (s *Source) FromProto(msg proto.Message) {
	source, ok := msg.(*rpc.Source)
	if !ok {
		panic(ErrInvalidProtoMessage)
	}

	name := source.GetName()

	s.ProjectID = source.GetProjectId()
	s.ID = MakeDocumentID(s.ProjectID, name)
	s.Name = name
	s.Description = source.GetDescription()
	s.CreatedAt = protoTimeToStd(source.GetCreatedAt())
	s.UpdatedAt = protoTimeToStd(source.GetUpdatedAt())

	switch source.Config.Config.(type) {
	case *rpc.SourceConfig_Gateway:
		s.Kind = SourceKindGateway
		s.Config.Gateway.FromProto(source.GetConfig().GetGateway())
	case *rpc.SourceConfig_File:
		s.Kind = SourceKindFile
		s.Config.File.FromProto(source.GetConfig().GetFile())
	default:
		panic(fmt.Sprintf("unexpected type: %+v", source.Config.Config))
	}
}

// ToProto converts s into *rpc.Source
func (s *Source) ToProto() proto.Message {
	createdAt := stdTimeToProto(s.CreatedAt)
	updatedAt := stdTimeToProto(s.UpdatedAt)

	config := &rpc.SourceConfig{}
	switch s.Kind {
	case SourceKindGateway:
		protoGw := s.Config.Gateway.ToProto()
		config.Config = &rpc.SourceConfig_Gateway{
			Gateway: protoGw.(*rpc.GatewaySource),
		}
	case SourceKindFile:
		protoFile := s.Config.File.ToProto()
		config.Config = &rpc.SourceConfig_File{
			File: protoFile.(*rpc.FileSource),
		}
	default:
		panic(fmt.Sprintf("unexpected config type: %d", s.Kind))
	}

	return &rpc.Source{
		Name:        s.Name,
		ProjectId:   s.ProjectID,
		Description: s.Description,
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
		Config:      config,
	}
}

// ToProtoResource converts s into a *rpc.Resource
func (s *Source) ToProtoResource() *rpc.Resource {
	return &rpc.Resource{
		Resource: &rpc.Resource_Source{
			Source: s.ToProto().(*rpc.Source),
		},
	}
}

type SourceParserKind uint8

const (
	SourceParserKindJSON SourceParserKind = iota
	SourceParserKindTSV
)

type SourceParser interface {
	Kind() SourceParserKind
	ToProto() proto.Message
	FromProto(proto.Message)
}

// SourceParserJSON is the JSON implementation of a
// parser for a SourceFile
type SourceParserJSON struct {
	ContinueOnError bool
}

func (j *SourceParserJSON) Kind() SourceParserKind {
	return SourceParserKindJSON
}

func (j *SourceParserJSON) FromProto(msg proto.Message) {
	parser := msg.(*rpc.FormatParser)
	json := parser.GetParser().(*rpc.FormatParser_Json).Json
	j.ContinueOnError = json.GetContinueOnError()
}

func (j *SourceParserJSON) ToProto() proto.Message {
	return &rpc.FormatParser{
		Parser: &rpc.FormatParser_Json{
			Json: &rpc.JsonParser{
				ContinueOnError: j.ContinueOnError,
			}},
	}
}

// SourceParserTSV is the TSV implementation of a
// parser for a SourceFile
type SourceParserTSV struct {
	Separator string
}

func (j *SourceParserTSV) Kind() SourceParserKind {
	return SourceParserKindTSV
}

func (t *SourceParserTSV) FromProto(msg proto.Message) {
	parser := msg.(*rpc.FormatParser)
	tsv := parser.GetParser().(*rpc.FormatParser_Tsv).Tsv
	t.Separator = tsv.GetSeparator()
}

func (j *SourceParserTSV) ToProto() proto.Message {
	return &rpc.FormatParser{
		Parser: &rpc.FormatParser_Tsv{
			Tsv: &rpc.TSVParser{
				Separator: j.Separator,
			}},
	}
}

type SourceParserUnion struct {
	JSON SourceParserJSON
	TSV  SourceParserTSV
}

type SourceFile struct {
	Path       string
	ParserKind SourceParserKind
	Parser     SourceParserUnion
}

func (SourceFile) Kind() SourceKind {
	return SourceKindFile
}

func (f *SourceFile) FromProto(msg proto.Message) {
	file, ok := msg.(*rpc.FileSource)
	if !ok {
		panic(ErrInvalidProtoMessage)
	}

	if len(file.Path) == 0 {
		panic(ErrInvalidProtoMessage)
	}

	f.Path = file.Path
	switch file.Parser.Parser.(type) {
	case *rpc.FormatParser_Json:
		f.ParserKind = SourceParserKindJSON
		f.Parser.JSON.FromProto(file.Parser)
	case *rpc.FormatParser_Tsv:
		f.ParserKind = SourceParserKindTSV
		f.Parser.TSV.FromProto(file.Parser)
	}
}

func (f *SourceFile) ToProto() proto.Message {
	var parser *rpc.FormatParser

	switch f.ParserKind {
	case SourceParserKindJSON:
		parser = f.Parser.JSON.ToProto().(*rpc.FormatParser)
	case SourceParserKindTSV:
		parser = f.Parser.TSV.ToProto().(*rpc.FormatParser)
	}

	return &rpc.FileSource{
		Path:   f.Path,
		Parser: parser,
	}
}

// SourceGateway is a rpc.GatewaySource
type SourceGateway struct {
	Type      SourceConnectorType
	Connector SourceConnectorUnion
}

// Kind of source for SourceGateway
func (SourceGateway) Kind() SourceKind {
	return SourceKindGateway
}

// FromProto panics if msg is not *rpc.GatewaySource
func (s *SourceGateway) FromProto(msg proto.Message) {
	gw, ok := msg.(*rpc.GatewaySource)
	if !ok {
		panic(ErrInvalidProtoMessage)
	}

	switch gw.Connector.Connector.(type) {
	case *rpc.Connector_Jdbc:
		s.Type = JdbcConnectorType

		jdbc := gw.GetConnector().GetJdbc()
		s.Connector.Jdbc = JdbcConnector{
			URL:            jdbc.GetUrl(),
			Driver:         jdbc.GetDriver(),
			User:           jdbc.GetUser(),
			Password:       jdbc.GetPassword(),
			IgnoreUserMode: jdbc.GetIgnoreUserMode(),
		}
	case *rpc.Connector_Kafka:
		s.Type = KafkaConnectorType

		kafka := gw.GetConnector().GetKafka()
		cfg := protoMapToStdMap(kafka.GetConsumerConfig().GetFields())
		s.Connector.Kafka = KafkaConnector{
			ConsumerConfig:      cfg,
			KeyDeserializer:     kafka.GetKeyDeserializer(),
			ValueDeserializer:   kafka.GetValueDeserializer(),
			IgnoreParsingErrors: kafka.GetIgnoreParsingErrors(),
		}
	case *rpc.Connector_Zoql:
		s.Type = ZOQLConnectorType

		zoql := gw.GetConnector().GetZoql()
		s.Connector.ZOQL = ZOQLConnector{
			User:      zoql.GetUser(),
			Password:  zoql.GetPassword(),
			Host:      zoql.GetHost(),
			FetchSize: zoql.GetFetchSize(),
		}
	case *rpc.Connector_Elastic:
		s.Type = ElasticSearchConnectorType

		es := gw.GetConnector().GetElastic()
		s.Connector.Elastic = ElasticSearchConnector{
			Host: es.GetHost(),
			Port: es.GetPort(),
		}
	case *rpc.Connector_Synthetic:
		s.Type = SyntheticConnectorType

		ss := gw.GetConnector().GetSynthetic()
		schema := protoMapToStdMap(ss.GetSchema().GetFields())
		s.Connector.Synthetic = SyntheticConnector{
			Seed:          ss.GetSeed(),
			Size:          ss.GetSize(),
			ProgressDelay: ss.GetProgressDelayMs(),
			Indexed:       ss.GetIndexed(),
			Schema:        schema,
		}
	case *rpc.Connector_Sonic:
		s.Type = SonicConnectorType

		ss := gw.GetConnector().GetSonic()
		config := protoMapToStdMap(ss.GetConfig().GetFields())
		s.Connector.Sonic = SonicConnector{
			Host:   ss.GetHost(),
			Port:   ss.GetPort(),
			Config: config,
		}
	case *rpc.Connector_Presto:
		s.Type = PrestoConnectorType

		p := gw.GetConnector().GetPresto()
		s.Connector.Presto = PrestoConnector{
			Host: p.GetHost(),
			Port: p.GetPort(),
		}
	default:
		panic(fmt.Sprintf("unexpected connector type: %+v",
			gw.Connector.Connector))
	}
}

// ToProto returns s as *rpc.GatewaySource
func (s SourceGateway) ToProto() proto.Message {
	connector := &rpc.Connector{}

	switch s.Type {
	case JdbcConnectorType:
		jdbc := s.Connector.Jdbc
		connector.Connector = &rpc.Connector_Jdbc{
			Jdbc: &rpc.JdbcConnector{
				Url:            jdbc.URL,
				Driver:         jdbc.Driver,
				User:           jdbc.User,
				Password:       jdbc.Password,
				IgnoreUserMode: jdbc.IgnoreUserMode,
			},
		}
	case KafkaConnectorType:
		kafka := s.Connector.Kafka
		config := structpb.Struct{
			Fields: stdMapToProtoMap(kafka.ConsumerConfig),
		}
		connector.Connector = &rpc.Connector_Kafka{
			Kafka: &rpc.KafkaConnector{
				KeyDeserializer:     kafka.KeyDeserializer,
				ValueDeserializer:   kafka.ValueDeserializer,
				ConsumerConfig:      &config,
				IgnoreParsingErrors: kafka.IgnoreParsingErrors,
			},
		}
	case ZOQLConnectorType:
		zoql := s.Connector.ZOQL
		connector.Connector = &rpc.Connector_Zoql{
			Zoql: &rpc.ZOQLConnector{
				User:      zoql.User,
				Password:  zoql.Password,
				Host:      zoql.Host,
				FetchSize: zoql.FetchSize,
			},
		}
	case ElasticSearchConnectorType:
		es := s.Connector.Elastic
		connector.Connector = &rpc.Connector_Elastic{
			Elastic: &rpc.ElasticSearchConnector{
				Host: es.Host,
				Port: es.Port,
			},
		}
	case SyntheticConnectorType:
		ss := s.Connector.Synthetic
		schema := structpb.Struct{
			Fields: stdMapToProtoMap(ss.Schema),
		}
		connector.Connector = &rpc.Connector_Synthetic{
			Synthetic: &rpc.SyntheticConnector{
				Seed:            ss.Seed,
				Size:            ss.Size,
				ProgressDelayMs: ss.ProgressDelay,
				Indexed:         ss.Indexed,
				Schema:          &schema,
			},
		}
	case SonicConnectorType:
		sonic := s.Connector.Sonic
		config := structpb.Struct{
			Fields: stdMapToProtoMap(sonic.Config),
		}
		connector.Connector = &rpc.Connector_Sonic{
			Sonic: &rpc.SonicConnector{
				Config: &config,
				Host:   sonic.Host,
				Port:   sonic.Port,
			},
		}
	case PrestoConnectorType:
		presto := s.Connector.Presto
		connector.Connector = &rpc.Connector_Presto{
			Presto: &rpc.PrestoConnector{
				Host: presto.Host,
				Port: presto.Port,
			},
		}
	default:
		panic(fmt.Sprintf("unexpected connector type: %d", s.Type))
	}

	return &rpc.GatewaySource{
		Connector: connector,
	}
}

// SourceConnectorUnion is a rpc.Connector
type SourceConnectorUnion struct {
	Jdbc      JdbcConnector
	Kafka     KafkaConnector
	ZOQL      ZOQLConnector
	Elastic   ElasticSearchConnector
	Synthetic SyntheticConnector
	Sonic     SonicConnector
	Presto    PrestoConnector
}

// SourceConnectorType of a SourceConnectorUnion.
type SourceConnectorType uint8

const (
	// JdbcConnectorType is a SourceConnectorType of rpc.JdbcConnector
	JdbcConnectorType SourceConnectorType = iota

	// KafkaConnectorType is a SourceConnectorType of rpc.KafkaConnector
	KafkaConnectorType

	// ZOQLConnectorType is a SourceConnectorType
	// of rpc.ZuoraObjectQueryLanguageConnector
	ZOQLConnectorType

	// ElasticSearchConnectorType is a SourceConnectorType
	// of rpc.ElasticSearchConnector
	ElasticSearchConnectorType

	// SyntheticConnectorType is a SourceConnectorType of rpc.SyntheticConnector
	SyntheticConnectorType

	// SonicConnectorType is a SourceConnectorType of rpc.SonicConnector
	SonicConnectorType

	// PrestoConnectorType is a SourceConnectorType of rpc.PrestoConnector
	PrestoConnectorType
)

// JdbcConnector is a rpc.JdbcConnector
type JdbcConnector struct {
	User           string
	Password       string
	URL            string
	Driver         string
	IgnoreUserMode bool
}

// KafkaConnector is a rpc.KafkaConnector
type KafkaConnector struct {
	KeyDeserializer     string
	ValueDeserializer   string
	IgnoreParsingErrors bool
	ConsumerConfig      map[string]interface{}
}

// ZOQLConnector is a rpc.ZOQLConnector
type ZOQLConnector struct {
	User      string
	Password  string
	Host      string
	FetchSize int32
}

// ElasticSearchConnector is a rpc.ElasticSearchConnector
type ElasticSearchConnector struct {
	Host string
	Port int32
}

// SyntheticConnector is a rpc.SyntheticConnector
type SyntheticConnector struct {
	Seed          int32
	Size          int32
	ProgressDelay int32
	Indexed       bool
	Schema        map[string]interface{}
}

// SonicConnector is a rpc.SonicConnector
type SonicConnector struct {
	Host   string
	Port   int32
	Config map[string]interface{}
}

// PrestoConnector is a rpc.PrestoConnector
type PrestoConnector struct {
	Host string
	Port int32
}

// WithUpdateSourceConfig returns an updated slice of updates
// which updates a Source config.
func WithUpdateSourceConfig(
	in []document.Update, cfg *rpc.SourceConfig,
) []document.Update {
	// NOTE: we might leave some other config types populated,
	// but that should be fine for now as we're not concerned about the size
	// in storage (yet).
	switch cfg.Config.(type) {
	case *rpc.SourceConfig_Gateway:
		var gw SourceGateway
		gw.FromProto(cfg.GetGateway())
		return append(in,
			document.Update{FieldPath: []string{"Type"}, Value: SourceKindGateway},
			document.Update{FieldPath: []string{"Config", "Gateway"}, Value: gw},
		)
	default:
		panic(fmt.Sprintf("unexpected type: %+v", cfg.Config))
	}
}
