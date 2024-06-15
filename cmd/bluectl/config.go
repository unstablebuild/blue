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
package main

import "github.com/unstablebuild/blue/config"

const defaultReferenceConfig = `
auth:
  project-id: 1
  credentials-file:
release:
  collection: blue-release
issue:
  collection: blue-issue
password:
  collection: blue-secret
`

type authConfig struct {
	ProjectID       string `yaml:"project-id"`
	CredentialsFile string `yaml:"credentials-file"`
}

type collectionConfig struct {
	Collection string `yaml:"collection"`
}

type issueConfig struct {
	Collection string `yaml:"collection"`
	Author     string `yaml:"author"`
}

type cliConfig struct {
	Auth     authConfig       `yaml:"auth"`
	Release  collectionConfig `yaml:"release"`
	Issue    issueConfig      `yaml:"issue"`
	Password collectionConfig `yaml:"password"`
}

func sourceConfig(overridesConfigPath string) (
	*cliConfig, error,
) {
	provider, err := config.NewProvider(
		overridesConfigPath, defaultReferenceConfig)
	if err != nil {
		return nil, err
	}

	return sourceConfigFromProvider(provider)
}

func sourceConfigFromProvider(provider config.Provider) (
	c *cliConfig, err error,
) {
	c = new(cliConfig)
	err = config.ProviderGetSections(provider,
		config.Section{Name: "auth", Target: &c.Auth},
		config.Section{Name: "release", Target: &c.Release},
		config.Section{Name: "issue", Target: &c.Issue},
		config.Section{Name: "password", Target: &c.Password},
	)
	return
}
