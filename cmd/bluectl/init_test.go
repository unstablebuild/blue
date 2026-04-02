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

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	defaultReleaseBucket     = "blue-release"
	defaultIssueCollection   = "blue-issue"
	defaultSecretsCollection = "blue-secret"
)

func newTestInitializer(_ *testing.T, dirName string) (
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
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, err)

	b, err := io.ReadAll(f)
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
	expectedConfig.Release.Bucket = defaultReleaseBucket
	expectedConfig.Issue.Collection = defaultIssueCollection
	expectedConfig.Password.Collection = defaultSecretsCollection

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
		dirName, err := os.MkdirTemp("", testTempDirPrefix)
		require.NoError(t, err)
		testInitializerRun(t, dirName, "1234")
	})

	t.Run("overrides existing config", func(t *testing.T) {
		dirName, err := os.MkdirTemp("", testTempDirPrefix)
		require.NoError(t, err)

		f, err := os.Create(path.Join(dirName, configFile))
		require.NoError(t, err)

		var config cliConfig
		config.Auth.ProjectID = "4321"
		config.Release.Collection = defaultReleaseCollection
		config.Release.Bucket = defaultReleaseBucket
		config.Issue.Collection = defaultIssueCollection
		config.Password.Collection = defaultSecretsCollection
		require.NoError(t, encodeConfig(f, &config))
		require.NoError(t, f.Close())

		testInitializerRun(t, dirName, "1234")
	})

	t.Run("returns error if config is corrupted", func(t *testing.T) {
		dirName, err := os.MkdirTemp("", testTempDirPrefix)
		require.NoError(t, err)

		f, err := os.Create(path.Join(dirName, configFile))
		require.NoError(t, err)

		_, err = f.Write([]byte("\rfweklf$!\f\f\f\f{\n"))
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
