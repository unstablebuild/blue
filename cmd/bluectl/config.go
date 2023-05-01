package main

import "github.com/ernestrc/blue/config"

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

type cliConfig struct {
	Auth     authConfig       `yaml:"auth"`
	Release  collectionConfig `yaml:"release"`
	Issue    collectionConfig `yaml:"issue"`
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
