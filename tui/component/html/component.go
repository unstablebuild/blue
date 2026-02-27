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

package html

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/PuerkitoBio/goquery"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/unstablebuild/blue/tui/component/markdown"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// State represents the current state of the HTML component.
type State int32

const (
	// StateLoading indicates the component is fetching and converting HTML.
	StateLoading State = iota
	// StateLoaded indicates the HTML has been fetched, converted, and rendered.
	StateLoaded
	// StateCanceled indicates the fetch was canceled by the user.
	StateCanceled
	// StateError indicates the fetch failed with an error.
	StateError
)

// Component fetches HTML content from a URL, converts it to markdown,
// and renders it in a terminal.
//
// While loading, it displays a progress animation placeholder inside
// a [component.Async]. Once the fetch completes, the async component
// swaps in the rendered [markdown.Component].
//
// The component is immutable after construction: each instance
// represents a single URL fetch.
//
// It implements [component.Floating].
type Component struct {
	inner    component.Floating
	cancel   context.CancelFunc
	anim     *component.Animation
	state    atomic.Int32
	resolved atomic.Pointer[markdown.Component]
}

var _ component.Floating = (*Component)(nil)

// New creates a new HTML component that fetches content from the given
// URL and renders it as markdown. The component starts loading immediately.
func New(interrupter term.Interrupter, u *url.URL, opts ...Option) *Component {
	cfg := config{
		mdCfg:      markdown.DefaultConfig(),
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	c := &Component{}
	c.state.Store(int32(StateLoading))

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	frames, sequence := component.ProgressAnimationFrames()
	c.anim = component.NewAnimation(interrupter, frames, sequence, 0)
	placeholder := component.StaticFloating(c.anim, 2, 1)

	var pendingResolved atomic.Bool

	wrappedInterrupter := term.FuncInterrupter(func(ictx context.Context) error {
		if component.IsAsyncContext(ictx) {
			if pendingResolved.Load() {
				c.state.Store(int32(StateLoaded))
			}
		}
		return interrupter.Interrupt(ictx)
	})

	rawURL := u.String()
	c.inner = component.Async(
		wrappedInterrupter, placeholder, func() (component.Floating, error) {
			comp, err := fetch(ctx, cfg.httpClient, rawURL, cfg.mdCfg)
			if err != nil {
				if ctx.Err() != nil {
					c.state.Store(int32(StateCanceled))
					return component.NewStringWithConfig("󰜺  Request was canceled",
						component.StringConfig{Alignment: component.AlignmentCentered},
					), nil
				}
				c.state.Store(int32(StateError))
				return nil, err
			}
			c.resolved.Store(comp)
			pendingResolved.Store(true)
			return comp, nil
		})

	return c
}

// State returns the current state of the component.
func (c *Component) State() State {
	return State(c.state.Load())
}

// Resolved returns the underlying markdown component. It must only
// be called when [State] returns [StateLoaded].
func (c *Component) Resolved() *markdown.Component {
	return c.resolved.Load()
}

// Cancel cancels the current fetch. If a fetch is in progress, the
// component transitions to [StateCanceled] and displays a cancellation
// message.
func (c *Component) Cancel() {
	c.cancel()
}

// Resize updates the viewport dimensions.
func (c *Component) Resize(width, height int) {
	c.inner.Resize(width, height)
}

// Draw renders the component content.
func (c *Component) Draw(w term.Writer) {
	c.inner.Draw(w)
}

// Dimensions returns the ideal content dimensions.
func (c *Component) Dimensions() (width, height int) {
	return c.inner.Dimensions()
}

// Close releases all resources associated with the component.
func (c *Component) Close() (ret error) {
	c.cancel()
	if err := c.anim.Close(); err != nil {
		ret = errors.Join(ret, err)
	}
	if inner, ok := c.inner.(io.Closer); ok {
		if err := inner.Close(); err != nil {
			ret = errors.Join(ret, err)
		}
	}
	return nil
}

func fetch(ctx context.Context, client *http.Client, rawURL string, mdCfg markdown.Config) (*markdown.Component, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	baseURL := resp.Request.URL
	converter := md.NewConverter("", true, &md.Options{
		GetAbsoluteURL: func(_ *goquery.Selection, ref string, _ string) string {
			if baseURL == nil {
				return ref
			}
			parsed, err := url.Parse(ref)
			if err != nil {
				return ref
			}
			return baseURL.ResolveReference(parsed).String()
		},
	})
	converter.Use(plugin.GitHubFlavored())

	buf, err := converter.ConvertReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return markdown.NewWithConfig(buf.String(), mdCfg)
}
