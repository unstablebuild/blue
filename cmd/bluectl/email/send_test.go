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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/emailprovider"
)

type recordingSender struct {
	calls    int
	messages []emailprovider.Message
	results  []emailprovider.Result
	err      error
}

func (s *recordingSender) Send(
	_ context.Context,
	messages []emailprovider.Message,
) ([]emailprovider.Result, error) {
	s.calls++
	s.messages = append(s.messages, messages...)
	return s.results, s.err
}

func TestSendRendersOneRecipient(t *testing.T) {
	path := writeTemplate(t, `<p>Hello {{.Name}}: {{.Recipient}}</p>`)
	provider := &recordingSender{results: []emailprovider.Result{{
		Status: emailprovider.StatusAccepted,
	}}}
	var gotSender emailprovider.Address
	var gotReplyTo *emailprovider.Address
	var gotUnsubscribeGroupID int
	command := newSendCLI(func(
		sender emailprovider.Address,
		replyTo *emailprovider.Address,
		unsubscribeGroupID int,
	) (emailprovider.Sender, error) {
		gotSender = sender
		gotReplyTo = replyTo
		gotUnsubscribeGroupID = unsubscribeGroupID
		return provider, nil
	}, "Configured <configured@example.com>", "configured-reply@example.com", 12345)

	err := command.Run(context.Background(), []string{
		"-X", "Name=Ernest",
		"-S", "Blue <sender@example.com>",
		"-R", "reply@example.com",
		"-U", "33767",
		"Welcome",
		"Recipient <recipient@example.com>",
		path,
	})
	require.NoError(t, err)
	require.Len(t, provider.messages, 1)
	assert.Equal(t, 1, provider.calls)
	assert.Equal(t, emailprovider.Address{Name: "Blue", Email: "sender@example.com"}, gotSender)
	require.NotNil(t, gotReplyTo)
	assert.Equal(t, "reply@example.com", gotReplyTo.Email)
	assert.Equal(t, 33767, gotUnsubscribeGroupID)
	assert.Equal(t, "recipient@example.com", provider.messages[0].Recipient.Email)
	assert.Equal(t, "Welcome", provider.messages[0].Subject)
	assert.Equal(t,
		`<p>Hello Ernest: recipient@example.com</p>`,
		provider.messages[0].HTMLBody)
}

func TestSendMissingVariableSendsNoEmail(t *testing.T) {
	path := writeTemplate(t, `{{$root := .}}{{if .Show}}<p>Hello {{$root.Missing}}</p>{{end}}`)
	provider := new(recordingSender)
	factoryCalls := 0
	command := newSendCLI(func(
		emailprovider.Address,
		*emailprovider.Address,
		int,
	) (emailprovider.Sender, error) {
		factoryCalls++
		return provider, nil
	}, "sender@example.com", "", 0)

	err := command.Run(context.Background(), []string{
		"-X", "Show=",
		"Welcome",
		"recipient@example.com",
		path,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing template variables: Missing")
	assert.Zero(t, factoryCalls, "provider construction could have side effects")
	assert.Zero(t, provider.calls, "no email may be submitted after a render failure")
}

func TestSendRequiresExactlyOneRecipient(t *testing.T) {
	path := writeTemplate(t, `<p>Hello</p>`)
	command := newSendCLI(nil, "sender@example.com", "", 0)

	err := command.Run(context.Background(), []string{
		"Welcome",
		"one@example.com",
		"two@example.com",
		path,
	})
	assert.Error(t, err)
}

func TestSendUsesConfiguredEnvelopeDefaults(t *testing.T) {
	path := writeTemplate(t, `<p>Hello</p>`)
	provider := &recordingSender{results: []emailprovider.Result{{
		Status: emailprovider.StatusAccepted,
	}}}
	var gotSender emailprovider.Address
	var gotReplyTo *emailprovider.Address
	var gotUnsubscribeGroupID int
	command := newSendCLI(func(
		sender emailprovider.Address,
		replyTo *emailprovider.Address,
		unsubscribeGroupID int,
	) (emailprovider.Sender, error) {
		gotSender = sender
		gotReplyTo = replyTo
		gotUnsubscribeGroupID = unsubscribeGroupID
		return provider, nil
	}, "Blue <sender@example.com>", "Support <reply@example.com>", 33767)

	err := command.Run(context.Background(), []string{
		"Configured subject",
		"recipient@example.com",
		path,
	})
	require.NoError(t, err)
	assert.Equal(t, emailprovider.Address{Name: "Blue", Email: "sender@example.com"}, gotSender)
	require.NotNil(t, gotReplyTo)
	assert.Equal(t, emailprovider.Address{Name: "Support", Email: "reply@example.com"}, *gotReplyTo)
	assert.Equal(t, 33767, gotUnsubscribeGroupID)
	assert.Equal(t, "Configured subject", provider.messages[0].Subject)
}

func TestSendManualDisplaysConfiguredDefaults(t *testing.T) {
	command := newSendCLI(nil, "Blue <sender@example.com>", "reply@example.com", 33767)
	manual := command.Man()

	assert.Equal(t, "[options] <subject> <recipient> <template-file.tmpl>", manual.Synopsis)
	assert.Equal(t, "Blue <sender@example.com>", manual.Options.Lookup("S").DefValue)
	assert.Equal(t, "reply@example.com", manual.Options.Lookup("R").DefValue)
	assert.Equal(t, "33767", manual.Options.Lookup("U").DefValue)
	assert.Nil(t, manual.Options.Lookup("subject"))
	assert.Nil(t, manual.Options.Lookup("sender"))
	assert.Nil(t, manual.Options.Lookup("reply-to"))
}
