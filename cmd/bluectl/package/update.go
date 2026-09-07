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

package pack

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/release"
)

const (
	updateTimeout = 10 * time.Second
)

type packageUpdate struct {
	m         release.Manager
	fs        *cli.FlagSet
	mdataFlag metadataFlag
}

func newReleaseUpdateCLI(m release.Manager) cli.CLI {
	c := &packageUpdate{
		m: m,
	}
	c.fs = cli.NewFlagSet("update")
	c.fs.Var(&c.mdataFlag, "d", "Merge metadata into the manifest. Expects format to be <key>=<value>")
	return c
}

func (s *packageUpdate) Man() cli.Manual {
	return cli.Manual{
		Name:     "update",
		Summary:  "Merge metadata into an existing package manifest",
		Synopsis: "<package>",
		Options:  *s.fs,
	}
}

func (s *packageUpdate) parseMetadataFlag() (map[string]string, error) {
	ret := make(map[string]string)
	for _, arg := range s.mdataFlag {
		kv := strings.Split(arg, "=")
		if len(kv) != 2 {
			return nil, errors.New("invalid -d format")
		}
		ret[kv[0]] = kv[1]
	}
	return ret, nil
}

func (s *packageUpdate) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	pack := args[0]

	mdata, err := s.parseMetadataFlag()
	if err != nil {
		cli.Usage(s)
		return err
	}
	if len(mdata) == 0 {
		cli.Usage(s)
		return errors.New("no metadata provided: pass at least one -d <key>=<value>")
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	return s.m.UpdatePackageMetadata(ctx, pack, mdata)
}
