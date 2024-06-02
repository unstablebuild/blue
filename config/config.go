package config

import (
	"io"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"go.uber.org/config"
)

// Provider is an abstraction over a configuration store
type Provider config.Provider

// Section represents a configuration section or sub-section within a file.
type Section struct {
	Name   string
	Target interface{}
}

// ProviderGetSections is a helper which takes a provider and series of sections
// and uses the Provider's API to populate the sections.
func ProviderGetSections(provider Provider, sections ...Section) (err error) {
	for _, section := range sections {
		err = provider.Get(section.Name).Populate(section.Target)
		if err != nil {
			return
		}
	}
	return
}

// NewProvider returns an instance of Provider by merging a
// reference config string literal and the contents of an override
// configuration file.
func NewProvider(overridesConfigPath, fallbackConfigLiteral string) (
	Provider, error,
) {
	configDef := strings.NewReader(fallbackConfigLiteral)
	if overridesConfigPath == "" {
		log.Warning("configuration file not defined, using defaults")
		//nolint:staticcheck
		return config.NewYAMLProviderFromReader(configDef)
	}
	configFile, err := os.Open(overridesConfigPath)
	if err != nil {
		return nil, err
	}
	//nolint:staticcheck
	return config.NewYAMLProviderFromReader(configDef, configFile)
}

// NewStringProvider returns a config Provider backed by a config string literal.
func NewStringProvider(stringConfig string) (Provider, error) {
	configDef := strings.NewReader(stringConfig)
	//nolint:staticcheck
	return config.NewYAMLProviderFromReader(configDef)
}

// NewFileProvider returns a config Provider backed by a config file.
func NewFileProvider(filePath string) (provider Provider, err error) {
	configFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	//nolint:staticcheck
	return config.NewYAMLProviderFromReader(configFile)
}

// NewReaderProvider returns a config Provider backed by a config io.Reader.
func NewReaderProvider(reader ...io.Reader) (provider Provider, err error) {
	//nolint:staticcheck
	return config.NewYAMLProviderFromReader(reader...)
}
