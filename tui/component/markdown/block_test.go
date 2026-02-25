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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

func TestHeaderBlockHeight(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		level    int
		content  textRun
		width    int
		expected int
	}{
		{
			name:     "simple header",
			level:    1,
			content:  textRun{{text: "Hello"}},
			width:    20,
			expected: 3, // 1 (above) + 1 (content) + 1 (spacing) = 3
		},
		{
			name:     "header wraps",
			level:    1,
			content:  textRun{{text: "Hello World"}},
			width:    8, // "# " takes 2 chars, leaving 6 for content
			expected: 4, // 1 (above) + 2 (wrapped lines) + 1 (spacing) = 4
		},
		{
			name:     "zero width",
			level:    1,
			content:  textRun{{text: "Hello"}},
			width:    0,
			expected: 0,
		},
		{
			name:     "h2 header",
			level:    2,
			content:  textRun{{text: "Title"}},
			width:    20,
			expected: 3, // 1 (above) + 1 (content) + 1 (spacing) = 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newHeaderBlock(tt.level, tt.content, &cfg)
			assert.Equal(t, tt.expected, block.Height(tt.width))
		})
	}
}

func TestParagraphBlockHeight(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		content  textRun
		width    int
		expected int
	}{
		{
			name:     "simple paragraph",
			content:  textRun{{text: "Hello"}},
			width:    20,
			expected: 2,
		},
		{
			name:     "paragraph wraps",
			content:  textRun{{text: "Hello World Test"}},
			width:    5,
			expected: 4,
		},
		{
			name:     "zero width",
			content:  textRun{{text: "Hello"}},
			width:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newParagraphBlock(tt.content, &cfg)
			assert.Equal(t, tt.expected, block.Height(tt.width))
		})
	}
}

func TestCodeBlockHeight(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		code     string
		width    int
		expected int
	}{
		{
			name:     "single line",
			code:     "code",
			width:    20,
			expected: 2,
		},
		{
			name:     "multiple lines",
			code:     "line1\nline2\nline3",
			width:    20,
			expected: 4,
		},
		{
			name:     "trailing newline",
			code:     "code\n",
			width:    20,
			expected: 2,
		},
		{
			name:     "zero width",
			code:     "code",
			width:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newCodeBlock("", tt.code, &cfg)
			assert.Equal(t, tt.expected, block.Height(tt.width))
		})
	}
}

func TestListBlockHeight(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		items    []listItem
		width    int
		expected int
	}{
		{
			name: "simple list",
			items: []listItem{
				{content: textRun{{text: "Item 1"}}},
				{content: textRun{{text: "Item 2"}}},
			},
			width:    20,
			expected: 3, // 2 items + 1 spacing at top level
		},
		{
			name: "nested list",
			items: []listItem{
				{
					content: textRun{{text: "Item 1"}},
					nested: newListBlock(false, 1, []listItem{
						{content: textRun{{text: "Nested"}}},
					}, &cfg),
				},
			},
			width:    20,
			expected: 3, // 1 parent item + 1 nested item + 1 spacing (only top level adds spacing)
		},
		{
			name:     "zero width",
			items:    []listItem{{content: textRun{{text: "Item"}}}},
			width:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newListBlock(false, 1, tt.items, &cfg)
			assert.Equal(t, tt.expected, block.Height(tt.width))
		})
	}
}

func TestBlockquoteBlockHeight(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		content  []textRun
		nested   *blockquoteBlock
		width    int
		expected int
	}{
		{
			name:     "simple blockquote",
			content:  []textRun{{{text: "Quote"}}},
			width:    20,
			expected: 2,
		},
		{
			name:    "nested blockquote",
			content: []textRun{{{text: "Outer"}}},
			nested: newBlockquoteBlock(
				[]textRun{{{text: "Inner"}}},
				nil,
				&cfg,
			),
			width:    20,
			expected: 4,
		},
		{
			name:     "zero width",
			content:  []textRun{{{text: "Quote"}}},
			width:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newBlockquoteBlock(tt.content, tt.nested, &cfg)
			assert.Equal(t, tt.expected, block.Height(tt.width))
		})
	}
}

func TestHorizontalRuleBlockHeight(t *testing.T) {
	cfg := DefaultConfig()
	block := newHorizontalRuleBlock(&cfg)

	assert.Equal(t, 2, block.Height(20))
	assert.Equal(t, 0, block.Height(0))
}

func TestTableBlockHeight(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		header   []textRun
		rows     [][]textRun
		width    int
		expected int
	}{
		{
			name:     "simple table",
			header:   []textRun{{{text: "A"}}, {{text: "B"}}},
			rows:     [][]textRun{{{{text: "1"}}, {{text: "2"}}}},
			width:    20,
			expected: 6, // top + header + sep + 1 row + bottom + spacing
		},
		{
			name:     "empty header",
			header:   nil,
			rows:     [][]textRun{{{{text: "1"}}, {{text: "2"}}}},
			width:    20,
			expected: 0,
		},
		{
			name:     "zero width",
			header:   []textRun{{{text: "A"}}},
			rows:     nil,
			width:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newTableBlock(tt.header, tt.rows, nil, &cfg)
			assert.Equal(t, tt.expected, block.Height(tt.width))
		})
	}
}

func TestBlockDraw(t *testing.T) {
	cfg := DefaultConfig()
	w := term.NewStringWriter(20, 10)

	tests := []struct {
		name         string
		block        block
		expectedRows int
	}{
		{
			name:         "header",
			block:        newHeaderBlock(1, textRun{{text: "Title"}}, &cfg),
			expectedRows: 3, // 1 (above) + 1 (content) + 1 (spacing)
		},
		{
			name:         "paragraph",
			block:        newParagraphBlock(textRun{{text: "Text"}}, &cfg),
			expectedRows: 2,
		},
		{
			name:         "code",
			block:        newCodeBlock("", "code", &cfg),
			expectedRows: 2,
		},
		{
			name:         "hrule",
			block:        newHorizontalRuleBlock(&cfg),
			expectedRows: 2,
		},
		{
			name:         "list",
			block:        newListBlock(false, 1, []listItem{{content: textRun{{text: "Item"}}}}, &cfg),
			expectedRows: 2,
		},
		{
			name:         "blockquote",
			block:        newBlockquoteBlock([]textRun{{{text: "Quote"}}}, nil, &cfg),
			expectedRows: 2,
		},
		{
			name: "table",
			block: newTableBlock(
				[]textRun{{{text: "A"}}},
				[][]textRun{{{{text: "1"}}}},
				nil,
				&cfg,
			),
			expectedRows: 6, // top + header + sep + 1 row + bottom + spacing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, w.Clear(term.Attributes{}))
			h := tt.block.Height(20)
			tt.block.Draw(w)
			assert.Equal(t, tt.expectedRows, h)
		})
	}
}

func TestTextRunString(t *testing.T) {
	tests := []struct {
		name     string
		run      textRun
		expected string
	}{
		{
			name:     "empty",
			run:      textRun{},
			expected: "",
		},
		{
			name:     "single span",
			run:      textRun{{text: "hello"}},
			expected: "hello",
		},
		{
			name:     "multiple spans",
			run:      textRun{{text: "hello "}, {text: "world"}},
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.run.String())
		})
	}
}

func TestTextRunLen(t *testing.T) {
	tests := []struct {
		name     string
		run      textRun
		expected int
	}{
		{
			name:     "empty",
			run:      textRun{},
			expected: 0,
		},
		{
			name:     "single span",
			run:      textRun{{text: "hello"}},
			expected: 5,
		},
		{
			name:     "multiple spans",
			run:      textRun{{text: "hello"}, {text: "world"}},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.run.Len())
		})
	}
}

func TestWrapTextRun(t *testing.T) {
	tests := []struct {
		name          string
		run           textRun
		width         int
		expectedLines int
	}{
		{
			name:          "fits in width",
			run:           textRun{{text: "hello"}},
			width:         10,
			expectedLines: 1,
		},
		{
			name:          "wraps at word boundary",
			run:           textRun{{text: "hello world"}},
			width:         6,
			expectedLines: 2,
		},
		{
			name:          "breaks long word",
			run:           textRun{{text: "superlongword"}},
			width:         5,
			expectedLines: 3,
		},
		{
			name:          "zero width",
			run:           textRun{{text: "hello"}},
			width:         0,
			expectedLines: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapTextRun(tt.run, tt.width)
			assert.Len(t, lines, tt.expectedLines)
		})
	}
}

func TestWrapTextRunPreservesSpaces(t *testing.T) {
	tests := []struct {
		name     string
		run      textRun
		width    int
		expected string
	}{
		{
			name:     "single span with spaces",
			run:      textRun{{text: "hello world"}},
			width:    20,
			expected: "hello world",
		},
		{
			name: "multiple spans with trailing space",
			run: textRun{
				{text: "Create file in "},
				{text: "component/", style: styleCode},
			},
			width:    80,
			expected: "Create file in component/",
		},
		{
			name: "multiple spans without explicit space",
			run: textRun{
				{text: "hello"},
				{text: " "},
				{text: "world"},
			},
			width:    80,
			expected: "hello world",
		},
		{
			name: "inline code in sentence",
			run: textRun{
				{text: "Implement "},
				{text: "tui.Component", style: styleCode},
				{text: " interface"},
			},
			width:    80,
			expected: "Implement tui.Component interface",
		},
		{
			name: "bold word in sentence",
			run: textRun{
				{text: "This is "},
				{text: "bold", style: styleBold},
				{text: " text"},
			},
			width:    80,
			expected: "This is bold text",
		},
		{
			name: "Development Workflow header",
			run: textRun{
				{text: "Development Workflow"},
			},
			width:    80,
			expected: "Development Workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapTextRun(tt.run, tt.width)
			var buf strings.Builder
			for _, line := range lines {
				buf.WriteString(line.String())
			}
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestHeaderPrefixRendering(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		level    int
		content  string
		width    int
		expected string
	}{
		{
			// H1 has background in DefaultConfig, so content is offset by 1
			name:     "H1 with # prefix",
			level:    1,
			content:  "Title",
			width:    12,
			expected: " # Title",
		},
		{
			// H2-H6 don't have background, so no offset
			name:     "H2 with ## prefix",
			level:    2,
			content:  "Section",
			width:    15,
			expected: "## Section",
		},
		{
			name:     "H3 with ### prefix",
			level:    3,
			content:  "Subsection",
			width:    20,
			expected: "### Subsection",
		},
		{
			name:     "H6 with ###### prefix",
			level:    6,
			content:  "Deep",
			width:    20,
			expected: "###### Deep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newHeaderBlock(tt.level, textRun{{text: tt.content}}, &cfg)
			w := term.NewStringWriter(tt.width, 4)
			require.NoError(t, w.Clear(term.Attributes{}))
			block.Height(tt.width)
			block.Draw(w)
			require.NoError(t, w.Flush())

			// The header is drawn at Y=1 (with 1 line padding above)
			// Extract the content line (line 1)
			lines := strings.Split(w.String(), "\n")
			require.GreaterOrEqual(t, len(lines), 2)
			contentLine := strings.TrimRight(lines[1], " ")
			assert.Equal(t, tt.expected, contentLine)
		})
	}
}

func TestNestedListSpacing(t *testing.T) {
	cfg := DefaultConfig()

	// Create a nested list similar to a table of contents
	nestedList := newListBlock(false, 1, []listItem{
		{content: textRun{{text: "Sub 1"}}},
		{content: textRun{{text: "Sub 2"}}},
	}, &cfg)

	parentList := newListBlock(false, 1, []listItem{
		{content: textRun{{text: "Item 1"}}},
		{
			content: textRun{{text: "Item 2"}},
			nested:  nestedList,
		},
		{content: textRun{{text: "Item 3"}}},
	}, &cfg)

	// Expected height:
	// - Item 1: 1 line
	// - Item 2: 1 line
	// - Sub 1: 1 line (nested, no extra spacing)
	// - Sub 2: 1 line (nested, no extra spacing)
	// - Item 3: 1 line
	// - Top level spacing: 1 line
	// Total: 6 lines
	assert.Equal(t, 6, parentList.Height(30))
}

func TestHeaderDimensions(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name           string
		level          int
		content        string
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "H1 dimensions",
			level:          1,
			content:        "Title",
			expectedWidth:  7, // "# " (2) + "Title" (5)
			expectedHeight: 3, // 1 (above) + 1 (content) + 1 (spacing)
		},
		{
			name:           "H3 dimensions",
			level:          3,
			content:        "Test",
			expectedWidth:  8, // "### " (4) + "Test" (4)
			expectedHeight: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := newHeaderBlock(tt.level, textRun{{text: tt.content}}, &cfg)
			w, h := block.Dimensions()
			assert.Equal(t, tt.expectedWidth, w)
			assert.Equal(t, tt.expectedHeight, h)
		})
	}
}

func TestHeaderPadding(t *testing.T) {
	cfg := DefaultConfig()
	block := newHeaderBlock(1, textRun{{text: "Title"}}, &cfg)
	w := term.NewStringWriter(15, 3)
	require.NoError(t, w.Clear(term.Attributes{}))
	block.Height(15)
	block.Draw(w)
	require.NoError(t, w.Flush())

	lines := strings.Split(w.String(), "\n")
	require.Len(t, lines, 3)

	// Line 0: blank (1 line padding above)
	assert.Equal(t, "               ", lines[0])
	// Line 1: " # Title" (content with 1 cell left padding since H1 has background)
	assert.Equal(t, " # Title       ", lines[1])
	// Line 2: blank (standard spacing)
	assert.Equal(t, "               ", lines[2])
}

func TestHeaderWrappingWithPrefix(t *testing.T) {
	cfg := DefaultConfig()
	// Test that long header content wraps correctly with prefix
	// Width = 12, prefix "## " = 3 chars, effective content width = 9 chars
	// "Long Title Here" wraps as: "Long", "Title", "Here" (each word on its own line at width 9)
	block := newHeaderBlock(2, textRun{{text: "Long Title Here"}}, &cfg)
	w := term.NewStringWriter(12, 5)
	require.NoError(t, w.Clear(term.Attributes{}))
	block.Height(12)
	block.Draw(w)
	require.NoError(t, w.Flush())

	lines := strings.Split(w.String(), "\n")
	require.Len(t, lines, 5)

	// Line 0: blank (1 line padding above)
	assert.Equal(t, "            ", lines[0])
	// Line 1: "## Long" (first line with prefix)
	contentLine1 := strings.TrimRight(lines[1], " ")
	assert.Equal(t, "## Long", contentLine1)
	// Line 2: "   Title" (continuation, indented with 3 spaces for "## " prefix)
	contentLine2 := strings.TrimRight(lines[2], " ")
	assert.Equal(t, "   Title", contentLine2)
	// Line 3: "   Here" (continuation)
	contentLine3 := strings.TrimRight(lines[3], " ")
	assert.Equal(t, "   Here", contentLine3)
}

func TestInlineStyleNotAppliedToLeadingWhitespace(t *testing.T) {
	// Test that leading whitespace before inline code/links doesn't inherit the style
	// Note: The markdown parser outputs spans with trailing space when there's a
	// space between normal text and styled text (e.g. "Run `command`" becomes
	// [{text: "Run "}, {text: "command", style: code}])
	tests := []struct {
		name           string
		run            textRun
		width          int
		expectedStyles []inlineStyle // styles for each span in output
	}{
		{
			name: "inline code preceded by text with trailing space",
			run: textRun{
				{text: "Run "},                          // trailing space
				{text: "command", style: styleCode},
			},
			width: 80,
			// Should output: "Run" (none), " " (none), "command" (code)
			expectedStyles: []inlineStyle{styleNone, styleNone, styleCode},
		},
		{
			name: "link preceded by text with trailing space",
			run: textRun{
				{text: "Click "},                        // trailing space
				{text: "here", style: styleLink, url: "http://example.com"},
			},
			width: 80,
			// Should output: "Click" (none), " " (none), "here" (link)
			expectedStyles: []inlineStyle{styleNone, styleNone, styleLink},
		},
		{
			name: "styled span with leading space",
			run: textRun{
				{text: "Run"},
				{text: " command", style: styleCode},    // leading space in styled span
			},
			width: 80,
			// Should output: "Run" (none), " " (none), "command" (code)
			expectedStyles: []inlineStyle{styleNone, styleNone, styleCode},
		},
		{
			name: "multiple styled spans with spaces",
			run: textRun{
				{text: "Use "},
				{text: "bold", style: styleBold},
				{text: " and "},
				{text: "code", style: styleCode},
			},
			width: 80,
			// Should output: "Use" (none), " " (none), "bold" (bold), " " (none),
			// "and" (none), " " (none), "code" (code)
			expectedStyles: []inlineStyle{
				styleNone, styleNone, styleBold, styleNone,
				styleNone, styleNone, styleCode,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapTextRun(tt.run, tt.width)
			require.Len(t, lines, 1, "expected single line output")

			var actualStyles []inlineStyle
			for _, sp := range lines[0] {
				actualStyles = append(actualStyles, sp.style)
			}
			assert.Equal(t, tt.expectedStyles, actualStyles)
		})
	}
}

func TestInlineCodeSpaceNotStyled(t *testing.T) {
	// Specific test: space before inline code should NOT have code style
	// When markdown parser sees "Prefix `code`", it produces:
	// [{text: "Prefix "}, {text: "code", style: code}]
	run := textRun{
		{text: "Prefix "},                           // trailing space
		{text: "code", style: styleCode},
	}
	lines := wrapTextRun(run, 80)
	require.Len(t, lines, 1)

	// Find the space span
	var spaceSpan *span
	for i := range lines[0] {
		if lines[0][i].text == " " {
			spaceSpan = &lines[0][i]
			break
		}
	}

	require.NotNil(t, spaceSpan, "expected to find space span")
	assert.Equal(t, styleNone, spaceSpan.style, "space should have no style")
}

func TestLinkSpaceNotUnderlined(t *testing.T) {
	// Specific test: space before link should NOT have link style (underline)
	// When markdown parser sees "Click [here](url)", it produces:
	// [{text: "Click "}, {text: "here", style: link, url: "..."}]
	run := textRun{
		{text: "Click "},                            // trailing space
		{text: "here", style: styleLink, url: "http://test.com"},
	}
	lines := wrapTextRun(run, 80)
	require.Len(t, lines, 1)

	// Find the space span
	var spaceSpan *span
	for i := range lines[0] {
		if lines[0][i].text == " " {
			spaceSpan = &lines[0][i]
			break
		}
	}

	require.NotNil(t, spaceSpan, "expected to find space span")
	assert.Equal(t, styleNone, spaceSpan.style, "space should have no style (no underline)")
}

func TestHeaderPrefixConfig(t *testing.T) {
	tests := []struct {
		name         string
		headerPrefix bool
		level        int
		content      string
		width        int
		expectedText string
	}{
		{
			name:         "prefix enabled (default)",
			headerPrefix: true,
			level:        1,
			content:      "Title",
			width:        20,
			expectedText: "# Title",
		},
		{
			name:         "prefix disabled",
			headerPrefix: false,
			level:        1,
			content:      "Title",
			width:        20,
			expectedText: "Title",
		},
		{
			name:         "H2 prefix enabled",
			headerPrefix: true,
			level:        2,
			content:      "Section",
			width:        20,
			expectedText: "## Section",
		},
		{
			name:         "H2 prefix disabled",
			headerPrefix: false,
			level:        2,
			content:      "Section",
			width:        20,
			expectedText: "Section",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.HeaderPrefix = tt.headerPrefix
			// Clear background to avoid offset
			cfg.H1 = term.Attributes{}
			cfg.H2 = term.Attributes{}

			block := newHeaderBlock(tt.level, textRun{{text: tt.content}}, &cfg)
			w := term.NewStringWriter(tt.width, 4)
			require.NoError(t, w.Clear(term.Attributes{}))
			block.Height(tt.width)
			block.Draw(w)
			require.NoError(t, w.Flush())

			// Header content is at Y=1 (1 line padding above)
			lines := strings.Split(w.String(), "\n")
			require.GreaterOrEqual(t, len(lines), 2)
			contentLine := strings.TrimRight(lines[1], " ")
			assert.Equal(t, tt.expectedText, contentLine)
		})
	}
}

func TestHeaderPrefixDimensions(t *testing.T) {
	cfg := DefaultConfig()

	// With prefix enabled
	cfg.HeaderPrefix = true
	block := newHeaderBlock(2, textRun{{text: "Title"}}, &cfg)
	w, h := block.Dimensions()
	assert.Equal(t, 8, w)  // "## " (3) + "Title" (5)
	assert.Equal(t, 3, h)

	// With prefix disabled
	cfg.HeaderPrefix = false
	block = newHeaderBlock(2, textRun{{text: "Title"}}, &cfg)
	w, h = block.Dimensions()
	assert.Equal(t, 5, w)  // "Title" (5) only
	assert.Equal(t, 3, h)
}

func TestHeaderBackgroundPadding(t *testing.T) {
	// Test that header background has symmetric padding (1 cell on each side)
	// For "# Title" (7 chars), background should be:
	// - X=0: padding (bg only, no content)
	// - X=1-7: content "# Title" (with bg)
	// - X=8: padding (bg only, no content)
	cfg := DefaultConfig()
	cfg.H1 = term.Attributes{Bg: tcell.ColorRed}

	block := newHeaderBlock(1, textRun{{text: "Title"}}, &cfg)
	w := term.NewStringWriter(20, 4)
	require.NoError(t, w.Clear(term.Attributes{}))
	block.Height(20)
	block.Draw(w)
	require.NoError(t, w.Flush())

	// Content "# Title" is 7 chars, with 1 padding each side = 9 cells with bg
	// Header is drawn at Y=1 (1 line padding above)
	// Check that content starts at X=1 (leaving X=0 for left padding)
	lines := strings.Split(w.String(), "\n")
	require.GreaterOrEqual(t, len(lines), 2)

	contentLine := lines[1]
	// X=0 should be a space (left padding)
	assert.Equal(t, ' ', rune(contentLine[0]), "X=0 should be padding space")
	// X=1 should be '#' (start of content)
	assert.Equal(t, '#', rune(contentLine[1]), "X=1 should be start of content '#'")
	// Content "# Title" spans X=1 to X=7
	// X=8 should be a space (right padding)
	assert.Equal(t, ' ', rune(contentLine[8]), "X=8 should be padding space")
}
