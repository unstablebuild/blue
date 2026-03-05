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
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// Config defines the styling options for markdown rendering.
type Config struct {
	// Header styles (H1-H6)
	H1, H2, H3, H4, H5, H6 term.Attributes
	// HeaderPrefix controls whether headers display the '#' prefix (e.g., "# Title").
	// When true (default), headers show the markdown prefix to indicate nesting level.
	HeaderPrefix bool

	// Text styles
	Paragraph     term.Attributes
	Bold          term.Attributes
	Italic        term.Attributes
	Strikethrough term.Attributes

	// Code styles
	CodeBlock  term.Attributes
	InlineCode term.Attributes
	// Parser, when set, enables syntax highlighting inside fenced code blocks.
	Parser syntaxapi.Parser
	// ScheduleNextTick schedules a function to run on the next event-loop
	// tick. Highlighting runs asynchronously via this callback so that
	// Highlight I/O does not block construction. Defaults to a
	// synchronous call in DefaultConfig.
	ScheduleNextTick func(func()) bool

	// Link styles
	Link    term.Attributes
	LinkURL term.Attributes

	// Blockquote styling
	Blockquote       term.Attributes
	BlockquoteBorder rune

	// List styling
	ListBullet        rune
	TaskListChecked   rune
	TaskListUnchecked rune

	// Table styling
	TableCharSet TableCharSet

	// Horizontal rule styling
	HorizontalRule     rune
	HorizontalRuleAttr term.Attributes

	// Spacing
	ParagraphSpacing int

	// Search highlight styles
	SearchMatch   term.Attributes // non-current search matches
	SearchCurrent term.Attributes // current search match
}

// TableCharSet defines the characters used to draw table borders.
type TableCharSet struct {
	component.FrameUnionCharSet
	HeaderSeparator rune
	ColumnSeparator rune
	CrossJoin       rune
	TopJoin         rune
	BottomJoin      rune
}

// DefaultTableCharSet returns the default table character set.
func DefaultTableCharSet() TableCharSet {
	return TableCharSet{
		FrameUnionCharSet: component.DefaultFrameUnionCharSet(),
		HeaderSeparator:   '─',
		ColumnSeparator:   '│',
		CrossJoin:         '┼',
		TopJoin:           '┬',
		BottomJoin:        '┴',
	}
}

// DefaultConfig returns a Config with sensible defaults for terminal rendering.
func DefaultConfig() Config {
	magenta := term.Attributes{Fg: tcell.ColorFuchsia}
	magentaBold := term.Attributes{Fg: tcell.ColorFuchsia, Attrs: tcell.AttrBold}
	cyan := term.Attributes{Fg: tcell.ColorTeal}
	cyanUnderline := term.Attributes{Fg: tcell.ColorTeal, Attrs: tcell.AttrUnderline}
	codeblock := term.Attributes{Fg: tcell.ColorSilver, Bg: tcell.ColorGray}
	gray := term.Attributes{Fg: tcell.ColorGray}
	def := term.Attributes{Fg: tcell.ColorDefault}
	defBold := term.Attributes{Fg: tcell.ColorDefault, Attrs: tcell.AttrBold}
	defItalic := term.Attributes{Fg: tcell.ColorDefault, Attrs: tcell.AttrItalic}
	dimWhite := term.Attributes{Fg: tcell.ColorDimGray}
	dimWhiteStrike := term.Attributes{
		Fg: tcell.ColorDefault, Attrs: tcell.AttrStrikeThrough | tcell.AttrDim,
	}
	purpleBold := term.Attributes{Fg: tcell.ColorPurple, Attrs: tcell.AttrBold}
	purple := term.Attributes{Fg: tcell.ColorPurple}
	purpleDim := term.Attributes{Fg: tcell.ColorPurple, Attrs: tcell.AttrDim}

	title := term.Attributes{
		Fg:    tcell.ColorWhite,
		Bg:    tcell.ColorPurple,
		Attrs: tcell.AttrBold,
	}
	return Config{
		H1:           title,
		H2:           magentaBold,
		H3:           magenta,
		H4:           purpleBold,
		H5:           purple,
		H6:           purpleDim,
		HeaderPrefix: true,

		Paragraph:     def,
		Bold:          defBold,
		Italic:        defItalic,
		Strikethrough: dimWhiteStrike,

		CodeBlock:  codeblock,
		InlineCode: codeblock,

		Link:    cyanUnderline,
		LinkURL: cyan,

		Blockquote:       dimWhite,
		BlockquoteBorder: '│',

		ListBullet:        '•',
		TaskListChecked:   '☑',
		TaskListUnchecked: '☐',

		TableCharSet: DefaultTableCharSet(),

		HorizontalRule:     '─',
		HorizontalRuleAttr: gray,

		ScheduleNextTick: func(cb func()) bool { cb(); return true },

		ParagraphSpacing: 1,

		SearchMatch:   term.Attributes{Attrs: tcell.AttrReverse},
		SearchCurrent: term.Attributes{Bg: tcell.ColorYellow, Fg: tcell.ColorBlack},
	}
}
