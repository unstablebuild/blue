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

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cmd/bluectl/options"
	"github.com/unstablebuild/blue/config"
	"gopkg.in/yaml.v3"
)

const (
	configFile     = "config"
	configFileMode = 0o600
)

type initializer struct {
	configFolder string
	inputReader  io.Reader
	outputWriter io.Writer
}

// newInitializer returns an instance of cli.CLI that
// helps bootstrap blue.
func newInitializer(configFolder string) initializer {
	return initializer{
		configFolder: configFolder,
		inputReader:  os.Stdin,
		outputWriter: os.Stdout,
	}
}

func (i initializer) Man() cli.Manual {
	return cli.Manual{
		Name:    "init",
		Summary: "Initialize or reinitialize this CLI's configuration.",
	}
}

func existsPath(path string) (exists bool, isDir bool, err error) {
	var info os.FileInfo
	info, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
			return
		}
		return
	}
	isDir = info.IsDir()
	exists = true
	return
}

func encodeConfig(f io.Writer, c *cliConfig) (err error) {
	encoder := yaml.NewEncoder(f)
	err = encoder.Encode(c)
	return
}

func updateConfig(filePath string, auth authConfig) error {
	if err := os.Chmod(filePath, configFileMode); err != nil {
		return err
	}
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_RDWR, os.ModeAppend)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	provider, err := config.NewReaderProvider(strings.NewReader(defaultReferenceConfig), f)
	if err != nil {
		return err
	}

	c, err := sourceConfigFromProvider(provider)
	if err != nil {
		return err
	}

	err = f.Truncate(0)
	if err != nil {
		return err
	}

	c.Auth = auth
	return encodeConfig(f, c)
}

func createDefaultConfig(filePath string, auth authConfig) error {
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, configFileMode)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	provider, err := config.NewStringProvider(defaultReferenceConfig)
	if err != nil {
		return err
	}

	c, err := sourceConfigFromProvider(provider)
	if err != nil {
		return err
	}

	c.Auth = auth
	return encodeConfig(f, c)
}

func (i initializer) readLine(header string) (str string, err error) {
	_, _ = fmt.Fprint(i.outputWriter, header)
	r := bufio.NewReader(i.inputReader)
	str, err = r.ReadString('\n')
	if err == io.EOF {
		err = nil
	}
	if length := len(str); length > 0 {
		str = str[:length-1]
	}
	return
}

func (i initializer) ensureConfigFile(filePath string) (bool, error) {
	ok, isDir, err := options.PathExists(filePath)
	if err != nil {
		return false, err
	}

	if isDir {
		err = fmt.Errorf("config file '%s' is a directory", filePath)
		return false, err
	}

	return ok, nil
}

func getCLIConfigFile(configFolder string) string {
	return path.Join(configFolder, configFile)
}

func (i initializer) Run(ctx context.Context, _ []string) error {
	projectID, err := i.readLine("ProjectID:")
	if err != nil {
		return err
	}

	return i.Initialize(projectID)
}

func (i initializer) Initialize(projectID string) error {
	err := options.EnsureFolderExists(i.configFolder)
	if err != nil {
		return err
	}

	configFilePath := getCLIConfigFile(i.configFolder)
	exists, err := i.ensureConfigFile(configFilePath)
	if err != nil {
		return err
	}

	var auth authConfig
	auth.ProjectID = projectID

	if exists {
		return updateConfig(configFilePath, auth)
	}

	return createDefaultConfig(configFilePath, auth)
}

func (i initializer) FolderExists() (exists bool, err error) {
	exists, _, err = existsPath(i.configFolder)
	return
}
