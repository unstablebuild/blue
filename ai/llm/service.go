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

package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/unstablebuild/blue/iterator"
)

// ErrContextWindowExceeded is returned when the number of tokens in a request
// exceeds the context window of a model. Users are encouraged to retry
// with a reduced number of messages.
type ErrContextWindowExceeded struct {
	Count, Max int
}

// Error satisfies the error interface.
func (e *ErrContextWindowExceeded) Error() string {
	return fmt.Sprintf("model context window exceeded (%d, max is %d)", e.Count, e.Max)
}

// Unwrap satisfies the error interface.
func (e *ErrContextWindowExceeded) Unwrap() error {
	return nil
}

// Is satisfies the error interface.
func (e *ErrContextWindowExceeded) Is(target error) bool {
	_, ok := target.(*ErrContextWindowExceeded)
	return ok
}

// Service encapsulates communications with an AI-capabilities provider.
type Service interface {
	// CreateChatCompletion attempts to complete the given chat completion request
	// using the pre-configured model. This method should return ErrContextWindowExceeded
	// if a request exceeds the context window. Clients can reduce the number of
	// messages until ExceedsContextWindow returns false.
	CreateChatCompletion(
		ctx context.Context,
		request ChatCompletionRequest,
	) (iterator.Iterator[ChatCompletionResponse], error)

	// ExceedsContextWindow returns true if the given message slice exceeds
	// the pre-configured model's context window.
	ExceedsContextWindow([]ChatCompletionMessage) (bool, error)
}

// ChatCompletionRequest represents a request structure for chat completion API.
type ChatCompletionRequest struct {
	// A list of messages comprising the conversation so far.
	Messages []ChatCompletionMessage
}

// ChatMessagePartType is a type of ChatMessagePart.
type ChatMessagePartType uint8

const (
	// ChatMessagePartTypeText is a text ChatMessagePart.
	ChatMessagePartTypeText ChatMessagePartType = iota
	// ChatMessagePartTypeImageURL is an image URL ChatMessagePart.
	ChatMessagePartTypeImageURL
)

// ChatMessagePart represents either or both textual and image message,
// in the context of a ChatCompletionMessage.
type ChatMessagePart struct {
	Type     ChatMessagePartType
	Text     string
	ImageURL string
}

// NewChatMessagePartFromImageURL returns a ChatMessagePart from an image URL.
func NewChatMessagePartFromImageURL(url string) ChatMessagePart {
	return ChatMessagePart{
		Type:     ChatMessagePartTypeImageURL,
		ImageURL: url,
	}
}

// NewChatMessagePartFromImageURL converts an image.Image, into a stream-ready
// ChatMessagePart.
func NewChatMessagePartFromImage(img image.Image) (ChatMessagePart, error) {
	var b bytes.Buffer
	b.WriteString("data:image/png;base64,")
	writer := base64.NewEncoder(base64.StdEncoding, &b)
	err := png.Encode(writer, img)
	if err != nil {
		return ChatMessagePart{}, fmt.Errorf("png encode: %w", err)
	}
	return NewChatMessagePartFromImageURL(b.String()), nil
}

// ChatCompletionMessage is a message in a chat with an assistant llm.
type ChatCompletionMessage struct {
	// The role of the author of this message.
	Role Role
	// The contents of the message.
	Content string
	// OtherContent overrides Content to provide the ability
	// to add non-textual content to the chat context.
	OtherContent []ChatMessagePart

	// Metadata contains service-specific data.
	// Check the documentation of a service implementation
	// to know what type this Metadata will be.
	//
	// Only the last Metadata field in a ChatCompletionMessage stream
	// should be used. Implementations must ensure that the last
	// ChatCompletionMessage's metadata is complete.
	Metadata any

	// An optional name for the participant.
	// Provides the model information to differentiate between participants of the same role.
	Name string
}

// ChatCompletionResponse represents a response structure for chat completion API.
type ChatCompletionResponse struct {
	// A unique identifier for the chat completion.
	ID string
	// Time when the chat completion was created.
	Created time.Time
	Message ChatCompletionMessage
	// The reason the model stopped generating tokens. This will be stop if the model
	// hit a natural stop point or a provided stop sequence, length if the maximum number
	// of tokens specified in the request was reached, content_filter if content was
	// omitted due to a flag from our content filters, tool_calls if the model
	// called a tool, or function_call (deprecated) if the model called a function.
	FinishReason FinishReason
}

// Role is the role of the message author in a message stream.
type Role string

// List of roles assigned to the different messages.
const (
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// FinishReason is the reason why the message choice was returned.
type FinishReason string

const (
	// FinishReasonStop API returned complete message,
	// or a message terminated by one of the stop sequences provided via the stop parameter
	FinishReasonStop FinishReason = "stop"
	// FinishReasonLength Incomplete model output due to max_tokens parameter or token limit
	FinishReasonLength FinishReason = "length"
	// FinishReasonToolCall The model decided to use one of the tools provided.
	FinishReasonToolCall FinishReason = "tool_calls"
	// FinishReasonContentFilter Omitted content due to a flag from our content filters
	FinishReasonContentFilter FinishReason = "content_filter"
	// FinishReasonNull API response still in progress or incomplete
	FinishReasonNull FinishReason = "null"
)
