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
