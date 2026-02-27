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

// Package html provides a TUI handler that fetches and renders HTML pages
// as markdown with less-like keyboard navigation, mouse text selection,
// link handling, and fetch cancellation.
//
// While loading, the handler accepts Ctrl+C to cancel and q/Esc to
// exit. Once the content is loaded, it provides full interactive
// rendering with scrolling, selection, and link clicks. Clicking an
// http/https link triggers navigation to the new URL. Previously
// visited pages are cached so that navigating back does not require a
// new fetch.
package html
