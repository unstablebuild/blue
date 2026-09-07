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

package email

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviewRendersAndOpensFileURL(t *testing.T) {
	path := writeTemplate(t, `<p>{{.Greeting}}, {{.Recipient}}</p>`)
	var opened *url.URL
	command := newPreviewCLI(func(location *url.URL) error {
		opened = location
		return nil
	}).(*previewCLI)
	var output bytes.Buffer
	command.output = &output

	err := command.Run(context.Background(), []string{
		"-o",
		"-X", "Greeting=Hello",
		"recipient@example.com",
		path,
	})
	require.NoError(t, err)
	require.NotNil(t, opened)
	assert.Equal(t, "file", opened.Scheme)
	assert.Equal(t, "opened email preview "+opened.Path+"\n", output.String())
	t.Cleanup(func() { _ = os.Remove(opened.Path) })
	body, err := os.ReadFile(opened.Path)
	require.NoError(t, err)
	assert.Equal(t, `<p>Hello, recipient@example.com</p>`, string(body))
}

func TestPreviewGeneratesWithoutOpeningBrowser(t *testing.T) {
	templatePath := writeTemplate(t, `<p>{{.Recipient}}</p>`)
	openCalls := 0
	command := newPreviewCLI(func(*url.URL) error {
		openCalls++
		return nil
	}).(*previewCLI)
	var output bytes.Buffer
	command.output = &output

	err := command.Run(context.Background(), []string{
		"recipient@example.com",
		templatePath,
	})
	require.NoError(t, err)
	assert.Zero(t, openCalls)
	previewPath := strings.TrimSpace(strings.TrimPrefix(output.String(), "generated email preview "))
	require.NotEmpty(t, previewPath)
	t.Cleanup(func() { _ = os.Remove(previewPath) })
	body, err := os.ReadFile(previewPath)
	require.NoError(t, err)
	assert.Equal(t, `<p>recipient@example.com</p>`, string(body))
}

func TestPreviewMissingVariableDoesNotOpenBrowser(t *testing.T) {
	path := writeTemplate(t, `<p>{{.Missing}}</p>`)
	openCalls := 0
	command := newPreviewCLI(func(*url.URL) error {
		openCalls++
		return nil
	})

	err := command.Run(context.Background(), []string{"recipient@example.com", path})
	require.Error(t, err)
	assert.Zero(t, openCalls)
}

func TestPreviewManualDocumentsOpenFlag(t *testing.T) {
	manual := newPreviewCLI(nil).Man()

	openFlag := manual.Options.Lookup("o")
	require.NotNil(t, openFlag)
	assert.Equal(t, "false", openFlag.DefValue)
}
