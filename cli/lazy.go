// Copyright 2018-2026 Unstable Build, LLC
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

package cli

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// Lazy returns a CLI that lazily uses the given constructor to
// initialize a CLI when Run is called for the first time.
func Lazy(constructor func(context.Context) (CLI, error)) CLI {
	return &lazy{
		constructor: constructor,
	}
}

type lazy struct {
	constructor func(context.Context) (CLI, error)
	cli         CLI
}

func (l *lazy) Run(ctx context.Context, args []string) (err error) {
	if l.cli == nil {
		l.cli, err = l.constructor(ctx)
		if err != nil {
			return
		}
	}
	return l.cli.Run(ctx, args)
}

func (l *lazy) Man() Manual {
	var err error
	if l.cli == nil {
		l.cli, err = l.constructor(context.Background())
		if err != nil {
			log.Errorf("man: failed to build CLI: %v", err)
			return Manual{}
		}
	}
	return l.cli.Man()
}
