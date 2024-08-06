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
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
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

type secretCreate struct {
	store      *secretmanager.Service
	fs         *cli.FlagSet
	mdataFlag  metadataFlag
	noEncodeID bool
}

func newSecretCreateCLI(s *secretmanager.Service) cli.CLI {
	c := &secretCreate{
		store: s,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.Var(&c.mdataFlag, "d", "Add metadata to the secret. Expects format to be <key>=<value>")
	c.fs.BoolVar(&c.noEncodeID, "n", false, "Dont't encode passed ID and treat it as the final secret ID.")
	return c
}

func (s *secretCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create a secret. The final secret ID is printed to stdout.",
		Synopsis: "[options] <id>",
		Options:  *s.fs,
	}
}

func (s *secretCreate) parseMetadataFlag() (map[string]string, error) {
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

func (s *secretCreate) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	id := args[0]

	if !s.noEncodeID {
		id = auth.EncodeSecretID(id)
	}

	mdata, err := s.parseMetadataFlag()
	if err != nil {
		cli.Usage(s)
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	err = s.store.CreateSecret(ctx, id, mdata)
	if err == nil {
		fmt.Fprint(os.Stdout, id)
		fmt.Fprint(os.Stdout, "\n")
	}
	return err
}
