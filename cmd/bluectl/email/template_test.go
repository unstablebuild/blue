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
