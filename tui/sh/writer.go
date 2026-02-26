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

package sh

import (
	"bytes"
	"context"

	"github.com/unstablebuild/rune-go-sdk/component"
)

// lineWriter is an io.Writer that buffers bytes, splits on
// newlines, and sends each complete line to a channel as a
// component.Responsive. All output uses default attributes.
type lineWriter struct {
	ch  chan<- component.Responsive
	ctx context.Context
	buf bytes.Buffer
}

// Write appends p to the internal buffer, extracts complete
// lines (up to each '\n'), and sends each as a
// ResponsiveString to the channel. It returns the context
// error when the context is done so that callers (e.g.
// io.Copy inside mvdan/sh) stop writing promptly.
func (w *lineWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n := len(p)
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadBytes('\n')
		if err != nil {
			// No more complete lines; put the partial
			// data back into the buffer.
			w.buf.Write(line)
			break
		}
		// Trim the trailing newline.
		text := string(bytes.TrimRight(line, "\n"))
		w.send(text)
	}
	return n, nil
}

// flush sends any remaining partial line in the buffer.
func (w *lineWriter) flush() {
	if w.buf.Len() == 0 {
		return
	}
	w.send(w.buf.String())
	w.buf.Reset()
}

func (w *lineWriter) send(text string) {
	item := component.NewResponsiveString(
		text, component.StringResponsiveConfig{},
	)
	select {
	case w.ch <- item:
	case <-w.ctx.Done():
	}
}
