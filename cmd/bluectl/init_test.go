package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	testTempDirPrefix        = "test_blue"
	defaultReleaseCollection = "blue-release"
	defaultIssueCollection   = "blue-issue"
)

func newTestInitializer(t *testing.T, dirName string) (
	i initializer, reader *bytes.Buffer, writer *bytes.Buffer,
) {
	reader = new(bytes.Buffer)
	writer = new(bytes.Buffer)
	i = initializer{dirName, reader, writer}
	return
}

func assertConfigInitialized(
	t *testing.T, folder string, expected cliConfig,
) {
	fileName := path.Join(folder, configFile)

	_, err := os.Stat(fileName)
	require.NoError(t, err)

	f, err := os.Open(fileName)
	defer f.Close()
	require.NoError(t, err)

	b, err := ioutil.ReadAll(f)
	require.NoError(t, err)

	expectedB, err := yaml.Marshal(&expected)
	require.NoError(t, err)

	assert.Equal(t, string(expectedB), string(b))
}

func runInitializerWithProjectID(
	t *testing.T, i initializer, reader *bytes.Buffer,
	writer *bytes.Buffer, projectID string,
) {
	_, err := reader.Write([]byte(projectID + "\n"))
	require.NoError(t, err)

	require.NoError(t, i.Run(context.Background(), nil))

	assert.Equal(t, "ProjectID:", writer.String())
}

func testInitializerRun(t *testing.T, dirName string, projectID string) {
	i, reader, writer := newTestInitializer(t, dirName)

	runInitializerWithProjectID(t, i, reader, writer, projectID)

	var expectedConfig cliConfig
	expectedConfig.Auth.ProjectID = projectID
	expectedConfig.Release.Collection = defaultReleaseCollection
	expectedConfig.Issue.Collection = defaultIssueCollection

	assertConfigInitialized(t, i.configFolder, expectedConfig)
}

func TestInitializerRun(t *testing.T) {
	t.Run("creates new directory and config", func(t *testing.T) {
		baseDir := os.TempDir()
		dirName := path.Join(baseDir,
			fmt.Sprintf("test_blue_%d", time.Now().UnixNano()))
		testInitializerRun(t, dirName, "1234")
	})

	t.Run("created new config", func(t *testing.T) {
		dirName, err := ioutil.TempDir("", testTempDirPrefix)
		require.NoError(t, err)
		testInitializerRun(t, dirName, "1234")
	})

	t.Run("overrides existing config", func(t *testing.T) {
		dirName, err := ioutil.TempDir("", testTempDirPrefix)
		require.NoError(t, err)

		f, err := os.Create(path.Join(dirName, configFile))
		require.NoError(t, err)

		var config cliConfig
		config.Auth.ProjectID = "4321"
		config.Release.Collection = defaultReleaseCollection
		config.Issue.Collection = defaultIssueCollection
		require.NoError(t, encodeConfig(f, &config))
		require.NoError(t, f.Close())

		testInitializerRun(t, dirName, "1234")
	})

	t.Run("returns error if config is corrupted", func(t *testing.T) {
		dirName, err := ioutil.TempDir("", testTempDirPrefix)
		require.NoError(t, err)

		f, err := os.Create(path.Join(dirName, configFile))
		require.NoError(t, err)

		_, err = f.Write([]byte("\rfweklf$!{\n"))
		require.NoError(t, err)
		require.NoError(t, f.Close())

		i, _, _ := newTestInitializer(t, dirName)
		assert.Error(t, i.Run(context.Background(), nil))
	})
}

func TestFolderExists(t *testing.T) {
	baseDir := os.TempDir()
	dirName := path.Join(baseDir,
		fmt.Sprintf("test_blue_folder_%d", time.Now().UnixNano()))
	i, _, _ := newTestInitializer(t, dirName)

	exists, err := i.FolderExists()
	require.NoError(t, err)
	assert.False(t, exists)

	err = os.MkdirAll(dirName, os.ModePerm)
	require.NoError(t, err)

	exists, err = i.FolderExists()
	require.NoError(t, err)
	assert.True(t, exists)
}
