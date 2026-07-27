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

// Package sendgrid implements emailprovider.Sender with SendGrid's v3 Mail Send API.
package sendgrid

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/unstablebuild/blue/emailprovider"
)

const (
	defaultEndpoint     = "https://api.sendgrid.com/v3/mail/send"
	maxPersonalizations = 1000
	maxErrorBodySize    = 1 << 20
	defaultTimeout      = 30 * time.Second
)

// Credentials contains the secret used to authenticate SendGrid requests.
type Credentials struct {
	APIKey string
}

// Config contains message-level and transport settings for a Sender.
type Config struct {
	Sender   emailprovider.Address
	ReplyTo  *emailprovider.Address
	Endpoint string
	Client   *http.Client
}

// sender sends email through SendGrid.
type sender struct {
	apiKey   string
	sender   emailprovider.Address
	replyTo  *emailprovider.Address
	endpoint string
	client   *http.Client
}

type sgAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type personalization struct {
	To         []sgAddress       `json:"to"`
	Subject    string            `json:"subject"`
	CustomArgs map[string]string `json:"custom_args,omitempty"`
}

type content struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type mailRequest struct {
	Personalizations []personalization `json:"personalizations"`
	From             sgAddress         `json:"from"`
	ReplyTo          *sgAddress        `json:"reply_to,omitempty"`
	Content          []content         `json:"content"`
}

type indexedMessage struct {
	index   int
	message emailprovider.Message
}

type body struct {
	text string
	html string
}

type sendError struct {
	rejected bool
	err      error
}

func splitRequests(group []indexedMessage) [][]indexedMessage {
	requests := make([][]indexedMessage, 0, (len(group)+maxPersonalizations-1)/maxPersonalizations)
	seen := make(map[string]struct{}, maxPersonalizations)
	var request []indexedMessage
	for _, indexed := range group {
		recipient := indexed.message.Recipient.Email
		_, duplicate := seen[recipient]
		if len(request) == maxPersonalizations || duplicate {
			requests = append(requests, request)
			request = nil
			clear(seen)
		}
		request = append(request, indexed)
		seen[recipient] = struct{}{}
	}
	if len(request) != 0 {
		requests = append(requests, request)
	}
	return requests
}

func (e *sendError) Error() string {
	return e.err.Error()
}

func (e *sendError) Unwrap() error {
	return e.err
}

// New validates credentials and configuration and returns a SendGrid sender.
func New(creds Credentials, config Config) (emailprovider.Sender, error) {
	apiKey := strings.TrimSpace(creds.APIKey)
	if apiKey == "" {
		return nil, errors.New("sendgrid API key is required")
	}
	if err := validateAddress("sender", config.Sender); err != nil {
		return nil, err
	}
	if config.ReplyTo != nil {
		if err := validateAddress("reply-to", *config.ReplyTo); err != nil {
			return nil, err
		}
	}

	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	var replyTo *emailprovider.Address
	if config.ReplyTo != nil {
		copy := *config.ReplyTo
		replyTo = &copy
	}

	return &sender{
		apiKey:   apiKey,
		sender:   config.Sender,
		replyTo:  replyTo,
		endpoint: endpoint,
		client:   client,
	}, nil
}

func validateAddress(kind string, address emailprovider.Address) error {
	parsed, err := mail.ParseAddress(address.Email)
	if err != nil || parsed.Address != address.Email {
		return fmt.Errorf("invalid %s address %q", kind, address.Email)
	}
	return nil
}

func validateMessages(messages []emailprovider.Message) error {
	for i, message := range messages {
		if err := validateAddress("recipient", message.Recipient); err != nil {
			return fmt.Errorf("message %d: %w", i, err)
		}
		if strings.TrimSpace(message.Subject) == "" {
			return fmt.Errorf("message %d: subject is required", i)
		}
		if message.HTMLBody == "" && message.TextBody == "" {
			return fmt.Errorf("message %d: HTML or text body is required", i)
		}
	}
	return nil
}

func address(address emailprovider.Address) sgAddress {
	return sgAddress{Email: address.Email, Name: address.Name}
}

func (s *sender) request(messages []indexedMessage) mailRequest {
	request := mailRequest{
		From:    address(s.sender),
		Content: make([]content, 0, 2),
	}
	if s.replyTo != nil {
		replyTo := address(*s.replyTo)
		request.ReplyTo = &replyTo
	}
	if messages[0].message.TextBody != "" {
		request.Content = append(request.Content, content{
			Type: "text/plain", Value: messages[0].message.TextBody,
		})
	}
	if messages[0].message.HTMLBody != "" {
		request.Content = append(request.Content, content{
			Type: "text/html", Value: messages[0].message.HTMLBody,
		})
	}
	for _, indexed := range messages {
		message := indexed.message
		request.Personalizations = append(request.Personalizations, personalization{
			To:         []sgAddress{address(message.Recipient)},
			Subject:    message.Subject,
			CustomArgs: message.Metadata,
		})
	}
	return request
}

func (s *sender) sendRequest(ctx context.Context, messages []indexedMessage) (string, error) {
	body, err := json.Marshal(s.request(messages))
	if err != nil {
		return "", fmt.Errorf("encode SendGrid request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create SendGrid request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+s.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("send SendGrid request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBodySize))
	if err != nil {
		return "", fmt.Errorf("read SendGrid response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", &sendError{
			rejected: response.StatusCode >= http.StatusBadRequest &&
				response.StatusCode < http.StatusInternalServerError,
			err: fmt.Errorf("SendGrid returned %s: %s", response.Status,
				strings.TrimSpace(string(responseBody))),
		}
	}

	return response.Header.Get("X-Message-Id"), nil
}

// Send validates every message before making a request, groups messages with
// identical bodies, and submits at most 1,000 private personalizations per request.
func (s *sender) Send(ctx context.Context, messages []emailprovider.Message) ([]emailprovider.Result, error) {
	if err := validateMessages(messages); err != nil {
		return nil, err
	}
	results := make([]emailprovider.Result, len(messages))
	if len(messages) == 0 {
		return results, nil
	}

	groups := make(map[body][]indexedMessage)
	var order []body
	for i, message := range messages {
		results[i] = emailprovider.Result{
			Recipient: message.Recipient,
			Status:    emailprovider.StatusUnknown,
		}
		key := body{text: message.TextBody, html: message.HTMLBody}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], indexedMessage{index: i, message: message})
	}
	var sendErrors []error
	for _, key := range order {
		group := groups[key]
		for _, chunk := range splitRequests(group) {
			providerID, err := s.sendRequest(ctx, chunk)
			status := emailprovider.StatusUnknown
			if err == nil {
				status = emailprovider.StatusAccepted
			} else {
				var providerError *sendError
				if errors.As(err, &providerError) && providerError.rejected {
					status = emailprovider.StatusRejected
				}
			}
			for _, indexed := range chunk {
				result := &results[indexed.index]
				result.ProviderID = providerID
				result.Err = err
				result.Status = status
			}
			if err != nil {
				sendErrors = append(sendErrors, err)
			}
		}
	}

	return results, errors.Join(sendErrors...)
}

var _ emailprovider.Sender = (*sender)(nil)
