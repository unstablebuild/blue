// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
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
