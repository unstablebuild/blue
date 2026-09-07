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

package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/unstablebuild/blue/cli"
)

var (
	// compile-time variables
	Tag     = "development"
	Commit  = "HEAD"
	Version string
)

func init() {
	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func exitWithError(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		exitWithError(err)
	}

	ctl, err := newBlueCtl(path.Join(homeDir, ".bluectl"))
	if err != nil {
		exitWithError(err)
	}
	defer func() { _ = ctl.Close() }()

	err = cli.Run(context.Background(), ctl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
