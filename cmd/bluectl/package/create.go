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
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/release"
)

const (
	createTimeout = 10 * time.Second
)

type metadataFlag []string

func (i *metadataFlag) String() string {
	return ""
}

func (i *metadataFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type packageCreate struct {
	m         release.Manager
	fs        *cli.FlagSet
	mdataFlag metadataFlag
}

func getDefaultAuthor() string {
	u, err := user.Current()
	if err != nil {
		u = &user.User{Username: "unknown"}
	}
	h, err := os.Hostname()
	if err != nil {
		h = "unknown-host"
	}
	return fmt.Sprintf("%s@%s", u.Username, h)
}

func newReleaseCreateCLI(m release.Manager) cli.CLI {
	c := &packageCreate{
		m: m,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.Var(&c.mdataFlag, "d", "Add default metadata to manifest. Expects format to be <key>=<value>")
	return c
}

func (s *packageCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create a package to group releases",
		Synopsis: "<package>",
		Options:  *s.fs,
	}
}

func (s *packageCreate) parseMetadataFlag() (map[string]string, error) {
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

func (s *packageCreate) Run(ctx context.Context, args []string) error {
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

	var notes string
	for _, field := range []string{"notes", "Notes"} {
		notes, ok = mdata[field]
		if ok {
			delete(mdata, field)
			break
		}
	}

	m, err := tempPackage(pack, getDefaultAuthor(), mdata, notes)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	return s.m.Create(ctx, m)
}
