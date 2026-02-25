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
	"strings"
	"unicode"
)

// wrapTextRun wraps a textRun to fit within the given width.
// Returns the wrapped lines as a slice of textRuns.
func wrapTextRun(run textRun, width int) []textRun {
	if width <= 0 {
		return nil
	}

	var lines []textRun
	var currentLine textRun
	var currentWidth int
	var needSpace bool

	for _, sp := range run {
		words, hasLeadingSpace, hasTrailingSpace := splitIntoWordsWithSpaces(sp.text)

		if hasLeadingSpace && len(words) == 0 {
			needSpace = true
			continue
		}

		for i, word := range words {
			// Add space as a separate unstyled span so it doesn't inherit styled
			// attributes (like inline code background or link underline)
			if (needSpace || (hasLeadingSpace && i == 0)) && currentWidth > 0 {
				currentLine = append(currentLine, span{text: " ", style: styleNone})
				currentWidth++
			}
			wordLen := len(word)

			if currentWidth > 0 && currentWidth+wordLen > width {
				lines = append(lines, currentLine)
				currentLine = nil
				currentWidth = 0
				word = strings.TrimLeft(word, " ")
				wordLen = len(word)
			}

			if wordLen > width {
				for len(word) > 0 {
					remaining := width - currentWidth
					if remaining <= 0 {
						if len(currentLine) > 0 {
							lines = append(lines, currentLine)
						}
						currentLine = nil
						currentWidth = 0
						remaining = width
					}
					chunk := word
					if len(chunk) > remaining {
						chunk = word[:remaining]
					}
					currentLine = append(currentLine, span{
						text:  chunk,
						style: sp.style,
						url:   sp.url,
					})
					currentWidth += len(chunk)
					word = word[len(chunk):]
				}
			} else if wordLen > 0 {
				currentLine = append(currentLine, span{
					text:  word,
					style: sp.style,
					url:   sp.url,
				})
				currentWidth += wordLen
			}
			needSpace = false
		}
		if hasTrailingSpace {
			needSpace = true
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	return lines
}

// splitIntoWordsWithSpaces splits text into words, preserving spaces as part
// of the following word when they occur between words. Returns the words,
// whether there was a leading space, and whether there was a trailing space.
func splitIntoWordsWithSpaces(text string) (words []string, leadingSpace, trailingSpace bool) {
	var current strings.Builder
	var sawSpace bool
	var sawNonSpace bool

	for _, r := range text {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			sawSpace = true
			if !sawNonSpace {
				leadingSpace = true
			}
		} else {
			if sawSpace && sawNonSpace {
				current.WriteRune(' ')
			}
			sawSpace = false
			sawNonSpace = true
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	trailingSpace = sawSpace && sawNonSpace
	if !sawNonSpace && sawSpace {
		leadingSpace = true
		trailingSpace = true
	}
	return
}

// countWrappedLines returns the number of lines needed to display the run.
func countWrappedLines(run textRun, width int) int {
	if width <= 0 {
		return 0
	}
	lines := wrapTextRun(run, width)
	if len(lines) == 0 {
		return 1
	}
	return len(lines)
}
