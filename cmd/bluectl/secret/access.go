// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package secret

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ernestrc/sensible/pager"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/iterator"
)

const (
	accessTimeout = 10 * time.Second
)

type secretAccess struct {
	manager  *secretmanager.Service
	fs       *cli.FlagSet
	all      bool
	noPrompt bool
}

func newSecretAccessCLI(s *secretmanager.Service) cli.CLI {
	c := &secretAccess{
		manager: s,
	}
	c.fs = cli.NewFlagSet("access")
	c.fs.BoolVar(&c.all, "A", false, "Include all enabled secret versions, rather than the latest.")
	c.fs.BoolVar(&c.noPrompt, "y", false, "Do not prompt user, simply print the secret to stdout.")
	return c
}

func (s *secretAccess) Man() cli.Manual {
	return cli.Manual{
		Name:     "access",
		Summary:  "Access secret(s) payloads.",
		Synopsis: "[options] <id>",
		Options:  *s.fs,
	}
}

func (s *secretAccess) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	id := args[0]

	ctx, cancel := context.WithTimeout(ctx, accessTimeout)
	defer cancel()

	var res []secretmanager.SecretVersion
	if s.all {
		res, err = s.manager.AccessSecretVersions(ctx, id)
	} else {
		var latest secretmanager.SecretVersion
		latest, err = s.manager.AccessSecretLatest(ctx, id)
		if err == nil {
			res = []secretmanager.SecretVersion{latest}
		}
	}
	if err != nil {
		return err
	}

	if s.noPrompt {
		for _, sec := range res {
			fmt.Fprint(os.Stdout, string(sec.Payload))
		}
		return nil
	}

	// do not show payload in table
	t := cliformat.Table[secretmanager.SecretVersion]([]string{"ID", "Version", "State", "CreatedAt"})
	if err := t.Format(ctx, os.Stdout, iterator.FromSlice(res)); err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Do you want view the secret(s) payload now? [y/n]:")
	text, _ := reader.ReadString('\n')
	switch text {
	case "y\n", "Y\n":
		for _, sec := range res {
			err := pager.PageReader(bytes.NewReader(sec.Payload))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
