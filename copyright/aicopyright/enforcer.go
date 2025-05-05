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

package aicopyright

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"

	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/unstablebuild/blue/ai/llm"
	"github.com/unstablebuild/blue/ai/llm/openai"
	"github.com/unstablebuild/blue/copyright"
)

// NewOpenaiEnforcer returns a copyright.Enforcer backed by openai's
// GPT4o model.
func NewOpenaiEnforcer(openaiApiKey string) copyright.Enforcer {
	schema, err := jsonschema.GenerateSchemaForType(infringesResponse{})
	if err != nil {
		panic(fmt.Sprintf("could not generate schema for infringe type: %v", err))
	}
	config := openai.Config{
		Model: openai.GPT4o,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   "infringes-response",
				Schema: schema,
				Strict: true,
			},
		},
	}
	svc := openai.NewClient(openaiApiKey, config, openai.AvailableModels())
	return agent{svc: svc}
}

type agent struct {
	svc llm.Service
}

func (a agent) ImageInfringes(ctx context.Context, img image.Image) (
	bool, error,
) {
	part, err := llm.NewChatMessagePartFromImage(img)
	if err != nil {
		return false, fmt.Errorf("encode image url: %w", err)
	}
	return a.ImageAtURLInfringes(ctx, part.ImageURL)
}

func (a agent) ImageAtURLInfringes(ctx context.Context, url string) (
	bool, error,
) {
	req := llm.ChatCompletionRequest{
		Messages: []llm.ChatCompletionMessage{
			{
				Role: llm.RoleUser,
				OtherContent: []llm.ChatMessagePart{
					llm.NewChatMessagePartFromImageURL(url),
					llm.ChatMessagePart{
						Type: llm.ChatMessagePartTypeText,
						Text: "Does this image infringe copyright?",
					},
				},
			},
		},
	}
	it, err := a.svc.CreateChatCompletion(ctx, req)
	if err != nil {
		return false, fmt.Errorf("llm create chat completion: %w", err)
	}

	var finishReason llm.FinishReason
	var builder bytes.Buffer
	for i := 0; ; i++ {
		resp, ok := it.Next(ctx)
		if !ok {
			break
		}
		builder.WriteString(resp.Message.Content)
		finishReason = resp.FinishReason
	}

	if it.Err() != nil {
		return false, fmt.Errorf("stream: %w", it.Err())
	}

	if finishReason != llm.FinishReasonStop {
		return false, fmt.Errorf("unexpected finish reason: %s", finishReason)
	}

	var infringes infringesResponse
	err = json.Unmarshal(builder.Bytes(), &infringes)
	if err != nil {
		return false, fmt.Errorf("unmarshal infringe response: %w", err)
	}

	return infringes.Infringes, nil
}

type infringesResponse struct {
	Infringes bool `json:"infringes"`
}
