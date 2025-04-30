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


package openai

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/ai/llm"
)

// NOTE: this serves as an example of how to use the llm API backed by openai.
// Only enable when changes to the API or client are made
// or when upgrading the sdk dependency.
// Note that some of the content asserts might fail, as without
// `seed` and `system_fingerprint` funcionality, the output is never deterministic.
func TestCreateChatCompletion(t *testing.T) {
	t.SkipNow()

	token := os.Getenv("OPENAI_TESTING_KEY")

	t.Run("NewClient with empty model panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewClient(token, Config{}, AvailableModels())
		})
	})

	t.Run("sends a chat completion request, with no context", func(t *testing.T) {
		client := NewClient(token, Config{
			Model:       GPT3Dot5Turbo,
			Temperature: 0.1,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.ChatCompletionRequest{Messages: []llm.ChatCompletionMessage{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}}
		it, err := client.CreateChatCompletion(ctx, req)
		require.NoError(t, err)

		var finishReason llm.FinishReason
		var builder strings.Builder
		for i := 0; ; i++ {
			resp, ok := it.Next(ctx)
			if !ok {
				break
			}
			assert.NotZero(t, resp.ID)
			assert.WithinDuration(t, time.Now(), resp.Created, 1*time.Minute)
			assert.Equal(t, llm.RoleAssistant, resp.Message.Role)
			assert.Len(t, resp.Message.Metadata.(Metadata).ToolCalls, 0)
			builder.WriteString(resp.Message.Content)
			finishReason = resp.FinishReason
		}

		require.NoError(t, it.Err())
		assert.Contains(t, builder.String(), "How can I")
		assert.Equal(t, llm.FinishReasonStop, finishReason)
	})

	t.Run("sets MaxTokens from configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			MaxTokens: 10000, // force error, so we know that it is set
			Model:     GPT3Dot5Turbo,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.ChatCompletionRequest{Messages: []llm.ChatCompletionMessage{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}}
		_, err := client.CreateChatCompletion(ctx, req)
		require.Error(t, err)
	})

	t.Run("sets PresencePenalty from configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			PresencePenalty: -100, // force error, so we know that it is set
			Model:           GPT3Dot5Turbo,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.ChatCompletionRequest{Messages: []llm.ChatCompletionMessage{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}}
		_, err := client.CreateChatCompletion(ctx, req)
		require.Error(t, err)
	})

	t.Run("sets Temperature from configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			Temperature: -100, // force error, so we know that it is set
			Model:       GPT3Dot5Turbo,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.ChatCompletionRequest{Messages: []llm.ChatCompletionMessage{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}}
		_, err := client.CreateChatCompletion(ctx, req)
		require.Error(t, err)
	})

	t.Run("sends a chat completion request, with some context, with default configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			Model:       GPT3Dot5Turbo,
			Temperature: 0.1,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.ChatCompletionRequest{Messages: []llm.ChatCompletionMessage{
			{Role: "system", Content: "We are roleplaying and you are an evil AI agent."},
			{Role: llm.RoleUser, Content: "what is your purpouse?"},
		}}
		it, err := client.CreateChatCompletion(ctx, req)
		require.NoError(t, err)

		var finishReason llm.FinishReason
		var builder strings.Builder
		for i := 0; ; i++ {
			resp, ok := it.Next(ctx)
			if !ok {
				break
			}
			assert.NotZero(t, resp.ID)
			assert.WithinDuration(t, time.Now(), resp.Created, 1*time.Minute)
			assert.Equal(t, llm.RoleAssistant, resp.Message.Role)
			assert.Len(t, resp.Message.Metadata.(Metadata).ToolCalls, 0)
			builder.WriteString(resp.Message.Content)
			finishReason = resp.FinishReason
		}

		require.NoError(t, it.Err())
		assert.Contains(t, builder.String(), "chaos")
		assert.Equal(t, llm.FinishReasonStop, finishReason)
	})

	t.Run("uses tools provided", func(t *testing.T) {
		client := NewClient(token, Config{
			Tools: []Tool{
				{Type: ToolTypeFunction, Function: FunctionDefinition{
					Name:        "getCurrentWeather",
					Description: "Get the weather in location",
					Parameters: jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"location": {
								Type:        jsonschema.String,
								Description: "The city and state, e.g. San Francisco, CA",
							},
							"unit": {
								Type: jsonschema.String,
								Enum: []string{"celcius", "fahrenheit"},
							},
						},
						Required: []string{"location"},
					},
				}},
			},
			Model: GPT3Dot5Turbo,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.ChatCompletionRequest{Messages: []llm.ChatCompletionMessage{
			{Role: llm.RoleUser, Content: "what's the weather like in San Francisco right now?"},
		}}
		it, err := client.CreateChatCompletion(ctx, req)
		require.NoError(t, err)

		var last llm.ChatCompletionResponse
		for i := 0; ; i++ {
			var ok bool
			resp, ok := it.Next(ctx)
			if !ok {
				break
			}
			last = resp
			assert.NotZero(t, resp.ID)
			assert.WithinDuration(t, time.Now(), resp.Created, 1*time.Minute)
			assert.Equal(t, llm.RoleAssistant, resp.Message.Role)

			// first message
			require.NotNil(t, resp.Message.Metadata, i)
			require.Len(t, resp.Message.Metadata.(Metadata).ToolCalls, 1, "%+v", resp)
			calls := resp.Message.Metadata.(Metadata).ToolCalls
			assert.NotZero(t, calls[0].ID)
			assert.Equal(t, ToolTypeFunction, calls[0].Type)
			assert.Equal(t, "getCurrentWeather", calls[0].Function.Name)
		}

		// last response should contain the reason
		assert.Equal(t, llm.FinishReasonToolCall, last.FinishReason)

		require.NoError(t, it.Err())
		actualArgs := last.Message.Metadata.(Metadata).ToolCalls[0].Function.Arguments
		assert.Equal(t, `{
  "location": "San Francisco, CA"
}`, actualArgs)
		actualFunction := last.Message.Metadata.(Metadata).ToolCalls[0].Function.Name
		assert.Equal(t, "getCurrentWeather", actualFunction)
	})
}

func TestContextWindows(t *testing.T) {
	models := AvailableModels()
	models[GPT4] = 50

	t.Run("CreateCompletionRequest errors with ErrContextWindowExceeded", func(t *testing.T) {
		client := NewClient("", Config{
			Model: GPT4,
		}, models).(client)
		ctx := context.Background()
		msgs := makeMessageTokens(client, 51)

		// sut
		req := llm.ChatCompletionRequest{Messages: msgs}
		_, err := client.CreateChatCompletion(ctx, req)
		require.True(t, errors.Is(err, &llm.ErrContextWindowExceeded{}), err.Error())
	})

	t.Run("ExceedsContextWindow", func(t *testing.T) {
		client := NewClient("", Config{
			Model: GPT4,
		}, models).(client)
		msgs := makeMessageTokens(client, 51)

		// sut
		ok, err := client.ExceedsContextWindow(msgs)
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = client.ExceedsContextWindow(msgs[1:])
		require.NoError(t, err)
		require.False(t, ok)
	})
}

func TestCountTokens(t *testing.T) {
	for model := range AvailableModels() {
		t.Run(model, func(t *testing.T) {
			client := NewClient("", Config{
				Model: model,
			}, AvailableModels()).(client)
			text := "¡Hola mundo!"
			assert.Equal(t, 10, client.countTokens([]llm.ChatCompletionMessage{{Content: text}}))
		})
	}
}

func makeMessageTokens(c client, greaterThan int) []llm.ChatCompletionMessage {
	var msgs []llm.ChatCompletionMessage
	for i := 0; c.countTokens(msgs) < greaterThan; i++ {
		msgs = append(msgs, llm.ChatCompletionMessage{
			Content: strconv.Itoa(i), Role: llm.RoleUser,
		})
	}
	return msgs
}
