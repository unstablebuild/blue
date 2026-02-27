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

// Package html provides a TUI component that fetches HTML content from
// a URL, converts it to markdown using html-to-markdown, and renders it
// using the markdown component.
//
// While loading, the component displays a progress animation. Once loaded,
// it displays the converted markdown content. The component is immutable
// after construction: each instance represents a single URL fetch.
package html
