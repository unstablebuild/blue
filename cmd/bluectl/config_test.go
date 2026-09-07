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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceConfigReadsSendGridAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), configFile)
	require.NoError(t, os.WriteFile(path, []byte(`
email:
  sender: Blue <sender@example.com>
  reply-to: Support <reply@example.com>
  sendgrid:
    api-key: test-sendgrid-key
    unsubscribe-group-id: 33767
`), configFileMode))

	config, err := sourceConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "Blue <sender@example.com>", config.Email.Sender)
	assert.Equal(t, "Support <reply@example.com>", config.Email.ReplyTo)
	assert.Equal(t, "test-sendgrid-key", config.Email.SendGrid.APIKey)
	assert.Equal(t, 33767, config.Email.SendGrid.UnsubscribeGroupID)
}
