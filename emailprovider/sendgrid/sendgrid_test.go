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

package sendgrid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/emailprovider"
)

func testSender(t *testing.T, handler http.HandlerFunc) emailprovider.Sender {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	replyTo := emailprovider.Address{Name: "Support", Email: "reply@example.com"}
	s, err := New(Credentials{APIKey: "test-key"}, Config{
		Sender:             emailprovider.Address{Name: "Blue", Email: "sender@example.com"},
		ReplyTo:            &replyTo,
		UnsubscribeGroupID: 33767,
		Endpoint:           server.URL,
		Client:             server.Client(),
	})
	require.NoError(t, err)
	return s
}

func TestSendUsesPrivatePersonalizations(t *testing.T) {
	requestCount := 0
	var got mailRequest
	sender := testSender(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.Header().Set("X-Message-Id", "request-id")
		w.WriteHeader(http.StatusAccepted)
	})
	messages := []emailprovider.Message{
		{
			Recipient: emailprovider.Address{Email: "one@example.com"},
			Subject:   "One",
			HTMLBody:  "<p>Hello</p>",
			Metadata:  map[string]string{"campaign": "test"},
		},
		{
			Recipient: emailprovider.Address{Email: "two@example.com"},
			Subject:   "Two",
			HTMLBody:  "<p>Hello</p>",
		},
	}

	results, err := sender.Send(context.Background(), messages)
	require.NoError(t, err)
	assert.Equal(t, 1, requestCount)
	require.Len(t, got.Personalizations, 2)
	assert.Equal(t, sgAddress{Email: "sender@example.com", Name: "Blue"}, got.From)
	require.NotNil(t, got.ReplyTo)
	assert.Equal(t, "reply@example.com", got.ReplyTo.Email)
	require.NotNil(t, got.ASM)
	assert.Equal(t, 33767, got.ASM.GroupID)
	for _, personalization := range got.Personalizations {
		assert.Len(t, personalization.To, 1, "recipients must never see one another")
	}
	assert.Equal(t, "one@example.com", got.Personalizations[0].To[0].Email)
	assert.Equal(t, "Two", got.Personalizations[1].Subject)
	assert.Equal(t, map[string]string{"campaign": "test"}, got.Personalizations[0].CustomArgs)
	require.Len(t, got.Content, 1)
	assert.Equal(t, "text/html", got.Content[0].Type)
	require.Len(t, results, 2)
	for _, result := range results {
		assert.Equal(t, emailprovider.StatusAccepted, result.Status)
		assert.Equal(t, "request-id", result.ProviderID)
		assert.NoError(t, result.Err)
	}
}

func TestSendGroupsDifferentBodiesIntoSeparateRequests(t *testing.T) {
	requestCount := 0
	sender := testSender(t, func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusAccepted)
	})
	messages := []emailprovider.Message{
		{Recipient: emailprovider.Address{Email: "one@example.com"}, Subject: "One", HTMLBody: "one"},
		{Recipient: emailprovider.Address{Email: "two@example.com"}, Subject: "Two", HTMLBody: "two"},
	}

	results, err := sender.Send(context.Background(), messages)
	require.NoError(t, err)
	assert.Equal(t, 2, requestCount)
	assert.Len(t, results, 2)
}

func TestSendOmitsASMWithoutUnsubscribeGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request mailRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Nil(t, request.ASM)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	sender, err := New(Credentials{APIKey: "test-key"}, Config{
		Sender:   emailprovider.Address{Email: "sender@example.com"},
		Endpoint: server.URL,
		Client:   server.Client(),
	})
	require.NoError(t, err)

	_, err = sender.Send(context.Background(), []emailprovider.Message{{
		Recipient: emailprovider.Address{Email: "one@example.com"},
		Subject:   "One",
		HTMLBody:  "one",
	}})
	require.NoError(t, err)
}

func TestSendValidatesWholeBatchBeforeSending(t *testing.T) {
	requestCount := 0
	sender := testSender(t, func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusAccepted)
	})
	messages := []emailprovider.Message{
		{Recipient: emailprovider.Address{Email: "one@example.com"}, Subject: "One", HTMLBody: "one"},
		{Recipient: emailprovider.Address{Email: "invalid"}, Subject: "Two", HTMLBody: "two"},
	}

	_, err := sender.Send(context.Background(), messages)
	require.Error(t, err)
	assert.Zero(t, requestCount)
}

func TestSendSplitsDuplicateRecipientsAcrossRequests(t *testing.T) {
	requestCount := 0
	sender := testSender(t, func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusAccepted)
	})
	messages := []emailprovider.Message{
		{Recipient: emailprovider.Address{Email: "same@example.com"}, Subject: "One", HTMLBody: "same"},
		{Recipient: emailprovider.Address{Email: "same@example.com"}, Subject: "Two", HTMLBody: "same"},
	}

	results, err := sender.Send(context.Background(), messages)
	require.NoError(t, err)
	assert.Equal(t, 2, requestCount)
	assert.Len(t, results, 2)
}

func TestSendDistinguishesRejectedAndUnknown(t *testing.T) {
	for _, test := range []struct {
		name   string
		code   int
		status emailprovider.Status
	}{
		{name: "validation rejection", code: http.StatusBadRequest, status: emailprovider.StatusRejected},
		{name: "provider failure", code: http.StatusInternalServerError, status: emailprovider.StatusUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			sender := testSender(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.code)
				_, _ = w.Write([]byte(`{"errors":[{"message":"failed"}]}`))
			})

			results, err := sender.Send(context.Background(), []emailprovider.Message{{
				Recipient: emailprovider.Address{Email: "one@example.com"},
				Subject:   "One",
				HTMLBody:  "one",
			}})
			require.Error(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, test.status, results[0].Status)
			assert.Error(t, results[0].Err)
		})
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	_, err := New(Credentials{}, Config{Sender: emailprovider.Address{Email: "sender@example.com"}})
	assert.EqualError(t, err, "sendgrid API key is required")

	_, err = New(Credentials{APIKey: "key"}, Config{Sender: emailprovider.Address{Email: "invalid"}})
	assert.EqualError(t, err, `invalid sender address "invalid"`)

	_, err = New(Credentials{APIKey: "key"}, Config{
		Sender:             emailprovider.Address{Email: "sender@example.com"},
		UnsubscribeGroupID: -1,
	})
	assert.EqualError(t, err, "sendgrid unsubscribe group ID cannot be negative")
}
