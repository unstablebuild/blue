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

// Package markdown provides a terminal-based markdown rendering component.
//
// The component parses markdown content using goldmark and renders it
// with proper formatting for terminal display, supporting headers, paragraphs,
// lists (ordered, unordered, task), code blocks, blockquotes, tables,
// and horizontal rules.
//
// # Basic Usage
//
//	md := markdown.New("# Hello World\n\nThis is a **bold** statement.")
//	md.Resize(80, 24)
//	md.Draw(writer)
//
// # Scrolling
//
// The component implements component.ScrollableFloating, allowing it to be
// scrolled when content exceeds the viewport:
//
//	md.SeekDown() // scroll down one line
//	md.SeekUp()   // scroll up one line
//
// # Customization
//
// Use NewWithConfig to customize colors and styling:
//
//	cfg := markdown.DefaultConfig()
//	cfg.H1 = term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold}
//	md := markdown.NewWithConfig(content, cfg)
package markdown
