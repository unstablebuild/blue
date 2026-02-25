// Copyright 2026 Unstable Build, LLC.
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

package markdown

import (
	"testing"

	"github.com/unstablebuild/rune-go-sdk/term"
)

func FuzzParse(f *testing.F) {
	f.Add("# Hello")
	f.Add("Hello **world**")
	f.Add("- item\n- item2")
	f.Add("```go\ncode\n```")
	f.Add("> quote")
	f.Add("---")
	f.Add("| a | b |\n|---|---|\n| 1 | 2 |")
	f.Add("")
	f.Add("# H1\n\n## H2\n\n### H3")
	f.Add("*italic* **bold** `code` ~~strike~~")
	f.Add("[link](http://example.com)")

	cfg := DefaultConfig()
	f.Fuzz(func(t *testing.T, input string) {
		_, err := parse(input, &cfg)
		if err != nil {
			t.Skip()
		}
	})
}

func FuzzRender(f *testing.F) {
	f.Add("# Hello", 80, 24)
	f.Add("Hello **world**", 40, 10)
	f.Add("- item\n- item2", 20, 5)
	f.Add("", 10, 10)
	f.Add("# Title\n\nParagraph", 1, 1)
	f.Add("Long text that needs to wrap around", 5, 3)

	f.Fuzz(func(t *testing.T, input string, width, height int) {
		if width < 0 || height < 0 || width > 1000 || height > 1000 {
			t.Skip()
		}

		md, err := New(input)
		if err != nil {
			t.Skip()
		}
		md.Resize(width, height)

		if width > 0 && height > 0 {
			w := term.NewStringWriter(width, height)
			md.Draw(w)
		}
	})
}

func FuzzScroll(f *testing.F) {
	f.Add("# H1\n\n# H2\n\n# H3", 10)
	f.Add("Line\nLine\nLine\nLine\nLine\nLine\nLine\nLine\nLine\nLine\n", 5)
	f.Add("Short", 100)

	f.Fuzz(func(t *testing.T, input string, scrollOps int) {
		if scrollOps < 0 || scrollOps > 1000 {
			t.Skip()
		}

		md, err := New(input)
		if err != nil {
			t.Skip()
		}
		md.Resize(40, 5)

		for range scrollOps {
			if scrollOps%2 == 0 {
				md.SeekDown()
			} else {
				md.SeekUp()
			}
		}

		if md.SeekOffset() < 0 {
			t.Errorf("offset became negative: %d", md.SeekOffset())
		}
		if md.SeekOffset() > md.MaxSeekOffset() {
			t.Errorf("offset exceeded max: %d > %d", md.SeekOffset(), md.MaxSeekOffset())
		}
	})
}
