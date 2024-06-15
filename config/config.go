// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
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
