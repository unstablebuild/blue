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
package pack

import (
	"context"
	"fmt"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/release"
)

const (
	defaultDescribeTimeout = 10 * time.Minute
)

type releaseDescribe struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newReleaseDescribeCLI(m release.Manager) cli.CLI {
	return releaseDescribe{
		m:  m,
		fs: cli.NewFlagSet("describe"),
	}
}

func (s releaseDescribe) Man() cli.Manual {
	return cli.Manual{
		Name:     "describe",
		Summary:  "Describe a package",
		Synopsis: "<package>",
		Options:  *s.fs,
	}
}

func printablePackage(man release.Package) (string, error) {
	var m manifest
	m.fromModel(man)
	return m.toYAML()
}

func (s releaseDescribe) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	pack := args[0]

	ctx, cancel := context.WithTimeout(ctx, defaultDescribeTimeout)
	defer cancel()

	man, err := s.m.GetPackage(ctx, pack)
	if err != nil {
		return err
	}

	data, err := printablePackage(man)
	if err != nil {
		return err
	}
	fmt.Printf("\n---\n%s\n", data)

	return nil
}
