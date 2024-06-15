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
package issue

import (
	"context"
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/debug"
	"github.com/unstablebuild/blue/issue"
)

const (
	createTimeout = 10 * time.Second
)

type panicReportPanic struct {
	t       issue.Tracker
	fs      *cli.FlagSet
	version string
}

func newReportPanicCLI(version string, t issue.Tracker) cli.CLI {
	c := &panicReportPanic{
		t: t,
	}
	c.fs = cli.NewFlagSet("panic")
	return c
}

func (s *panicReportPanic) Man() cli.Manual {
	return cli.Manual{
		Name:     "panic",
		Summary:  "Create a bug report by capturing a simulated panic",
		Synopsis: "",
		Options:  *s.fs,
	}
}

func (s *panicReportPanic) Run(ctx context.Context, args []string) error {
	_, _, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	discard := log.New()
	discard.Out = io.Discard

	ok, report := debug.CapturePanic(discard, "blue", s.version, func() {
		panic("this is a simulation")
	})
	if ok {
		log.Fatal("expected panic report")
	}
	id, err := s.t.CreateReport(ctx, report)
	if err != nil {
		return err
	}
	fmt.Printf("Created issue %q", id)
	return nil
}
