package main

import (
	"github.com/ernestrc/blue/config"
)

const defaultReferenceConfig = `
grpc:
    tls: false
    cert-file: my_cert.cert
    key-file: my_key.pem
    port: 4238
gc:
    project-id: 1
    creds-file:
    collections:
        source: source-resource-dev
`

type collectionsConfig struct {
	Source string `yaml:"source"`
}

type gcConfig struct {
	ProjectID   string            `yaml:"project-id"`
	CredsFile   string            `yaml:"creds-file"`
	Collections collectionsConfig `yaml:"collections"`
}

type grpcConfig struct {
	Port     int32  `yaml:"port"`
	TLS      bool   `yaml:"tls"`
	CertFile string `yaml:"cert-file"`
	KeyFile  string `yaml:"key-file"`
}

type appConfig struct {
	GRPC grpcConfig
	GC   gcConfig
}

func sourceConfig(referenceConfigLiteral, overridesConfigPath string) (
	c *appConfig, err error,
) {
	provider, err := config.NewProvider(
		overridesConfigPath, referenceConfigLiteral)
	if err != nil {
		return
	}

	c = new(appConfig)

	config.ProviderGetSections(provider,
		config.Section{Name: "grpc", Target: &c.GRPC},
		config.Section{Name: "gc", Target: &c.GC},
	)

	return
}
