package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/cmd/bluectl/options"
	"github.com/ernestrc/blue/config"
	"gopkg.in/yaml.v3"
)

const configFile = "config"

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
		Summary: "Initialize or reinitialize blue",
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
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_RDWR, os.ModeAppend)
	if err != nil {
		return err
	}

	defer f.Close()

	provider, err := config.NewReaderProvider(f)
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
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer f.Close()

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
	fmt.Fprint(i.outputWriter, header)
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
