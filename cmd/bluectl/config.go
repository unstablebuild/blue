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
contributor:
  program: contributor-program
  participants: contributor-participants
  awards: contributor-awards
  receipts: contributor-receipts
  rounds: contributor-rounds
  obligations: contributor-obligations
  operator:
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

// contributorConfig names the Firestore collections backing the
// contributor program. They must match the ox-api
// --contributor-*-collection flags: bluectl and the API write the same
// ledger.
type contributorConfig struct {
	Program      string `yaml:"program"`
	Participants string `yaml:"participants"`
	Awards       string `yaml:"awards"`
	Receipts     string `yaml:"receipts"`
	Rounds       string `yaml:"rounds"`
	Obligations  string `yaml:"obligations"`

	// Operator identifies who runs the CLI. It is recorded as the
	// proposer, decider or closer of everything bluectl writes to the
	// ledger.
	Operator string `yaml:"operator"`
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
	Auth        authConfig        `yaml:"auth"`
	Release     releaseConfig     `yaml:"release"`
	Issue       issueConfig       `yaml:"issue"`
	Password    collectionConfig  `yaml:"password"`
	Newsletter  collectionConfig  `yaml:"newsletter"`
	Contributor contributorConfig `yaml:"contributor"`
	Email       emailConfig       `yaml:"email"`
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
		config.Section{Name: "contributor", Target: &c.Contributor},
		config.Section{Name: "email", Target: &c.Email},
	)
	return
}
