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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/emailprovider"
)

func writeTemplate(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "message.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestTemplateVariables(t *testing.T) {
	var variables templateVariables
	require.NoError(t, variables.Set("Link=https://example.com?a=b"))
	assert.Equal(t, "https://example.com?a=b", variables["Link"])

	require.NoError(t, variables.Set("Link=replaced"))
	assert.Equal(t, "replaced", variables["Link"])
	assert.EqualError(t, variables.Set("missing-delimiter"), "-X expects key=value")
	assert.EqualError(t, variables.Set("=missing-key"), "-X expects key=value")
	assert.EqualError(t, variables.Set("Recipient=override"), `-X variable "Recipient" is reserved`)
}

func TestRenderStrictVariablesAndRecipient(t *testing.T) {
	path := writeTemplate(t, `<p>Hello {{.Name}} at {{.Recipient}}</p>`)
	recipient := emailprovider.Address{Name: "A & B", Email: "team@example.com"}

	body, err := render(path, recipient, templateVariables{"Name": `<Admin>`})
	require.NoError(t, err)
	assert.Equal(t,
		`<p>Hello &lt;Admin&gt; at team@example.com</p>`,
		string(body))

	_, err = render(path, recipient, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing template variables: Name")
}

func TestRenderValidatesVariablesBeforeExecution(t *testing.T) {
	recipient := emailprovider.Address{Email: "team@example.com"}
	path := writeTemplate(t, `{{$root := .}}{{if .Show}}{{$root.Missing}}{{$.AlsoMissing}}{{($root).Chained}}{{end}} {{index ($root) "Indexed"}}`)

	_, err := render(path, recipient, templateVariables{"Show": ""})
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing template variables: AlsoMissing, Chained, Indexed, Missing")

	body, err := render(path, recipient, templateVariables{
		"Show":        "",
		"Missing":     "unused",
		"AlsoMissing": "unused",
		"Chained":     "unused",
		"Indexed":     "included",
	})
	require.NoError(t, err)
	assert.Equal(t, " included", string(body))
}

func TestRenderRejectsDynamicRootAliasesBeforeExecution(t *testing.T) {
	recipient := emailprovider.Address{Email: "team@example.com"}
	path := writeTemplate(t, `{{$maybeRoot := or . .}}{{if .Show}}{{$maybeRoot.Missing}}{{end}}`)

	_, err := render(path, recipient, templateVariables{"Show": ""})
	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot access fields on local variable $maybeRoot")
}
