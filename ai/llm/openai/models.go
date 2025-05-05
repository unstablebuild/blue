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

import "github.com/sashabaranov/go-openai"

const (
	// GPT3Dot5Turbo is a faster, more lightweight predecessor to GPT-4,
	// suited for everyday tasks with lower resource demands and lower
	// cost.
	GPT3Dot5Turbo = openai.GPT3Dot5Turbo1106

	// GPT4 is the original GPT-4 model known for its high reasoning abilities,
	// commonly used for professional and academic applications.
	GPT4 = openai.GPT4

	// GPT4Turbo is a faster and more cost-efficient variant of GPT-4
	// with high accuracy and long context support (up to 128k tokens),
	// optimized for performance.
	GPT4Turbo = openai.GPT4Turbo

	// O1 is the first in a new series of AI models designed
	// to "think before responding," employing internal chain-of-thought
	// reasoning to tackle complex tasks in science, mathematics, and programming.
	O1 = openai.O1

	// O1Mini is a cost-effective reasoning model optimized for STEM tasks,
	// particularly math and coding. Achieves performance comparable to the full o1
	// model on benchmarks like AIME and Codeforces, while being approximately
	// 80% more cost-efficient. Ideal for applications requiring reasoning without
	// extensive general world knowledge.​
	O1Mini = openai.O1Mini

	// O3 is the successor to O1, offering enhanced reasoning abilities and performance.
	O3 = openai.O3

	// O3Mini is an enhanced reasoning model offering faster and more
	// accurate responses in STEM domains compared to o1-mini. Demonstrates improved
	// accuracy and speed over o1-mini, with the "high" reasoning mode achieving
	// notable benchmarks in tasks like AIME and GPQA Diamond.
	O3Mini = openai.O3Mini

	// O4Mini excels in mathematics, coding, and visual tasks; surpasses its
	// predecessor, o3-mini, in both STEM and non-STEM domains like data science.
	// Seamlessly utilizes ChatGPT tools such as web browsing, Python execution,
	// image analysis, and file interpretation, enabling autonomous multi-step reasoning.
	// Capable of interpreting and reasoning with images, including sketches
	// and diagrams, by integrating them into its thought process.
	// Offers a significant reduction in operational costs compared to larger models,
	// making it suitable for high-throughput applications.
	// Supports up to 200,000 tokens, facilitating extended interactions and
	// complex problem-solving.​
	O4Mini = openai.O4Mini

	// GPT4o is OpenAI's latest flagship model offering improved speed,
	// lower cost, and native multimodal capabilities (text, vision, audio)
	// in one unified model.
	GPT4o = openai.GPT4o
)

// AvailableModels returns a set with the available models and their
// corresponding maximum context windows in tokens.
func AvailableModels() (ret map[string]int) {
	return map[string]int{
		GPT3Dot5Turbo: 16000,
		GPT4:          8192,
		GPT4Turbo:     128000,
		O1:            32768,
		O1Mini:        32768,
		O3:            65536,
		O3Mini:        65536,
		O4Mini:        200000,
		GPT4o:         128000,
	}
}
