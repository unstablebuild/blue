// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "github.com/unstablebuild/blue/config"

const defaultReferenceConfig = `
auth:
  project-id: 1
  credentials-file:
release:
  collection: blue-release
  bucket: blue-release
issue:
  collection: blue-issue
password:
  collection: blue-secret
newsletter:
  collection: newsletter-subscribers
email:
  sender:
  reply-to:
  sendgrid:
    api-key:
    unsubscribe-group-id: 0
`

type authConfig struct {
	ProjectID       string `yaml:"project-id"`
	CredentialsFile string `yaml:"credentials-file"`
}

type collectionConfig struct {
	Collection string `yaml:"collection"`
}

type releaseConfig struct {
	Collection string `yaml:"collection"`
	Bucket     string `yaml:"bucket"`
}

type issueConfig struct {
	Collection string `yaml:"collection"`
	Author     string `yaml:"author"`
}

type sendgridConfig struct {
	APIKey             string `yaml:"api-key"`
	UnsubscribeGroupID int    `yaml:"unsubscribe-group-id"`
}

type emailConfig struct {
	Sender   string         `yaml:"sender"`
	ReplyTo  string         `yaml:"reply-to"`
	SendGrid sendgridConfig `yaml:"sendgrid"`
}

type cliConfig struct {
	Auth       authConfig       `yaml:"auth"`
	Release    releaseConfig    `yaml:"release"`
	Issue      issueConfig      `yaml:"issue"`
	Password   collectionConfig `yaml:"password"`
	Newsletter collectionConfig `yaml:"newsletter"`
	Email      emailConfig      `yaml:"email"`
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
		config.Section{Name: "newsletter", Target: &c.Newsletter},
		config.Section{Name: "email", Target: &c.Email},
	)
	return
}
