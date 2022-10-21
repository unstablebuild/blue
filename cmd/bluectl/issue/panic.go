package issue

import (
	"context"
	"io/ioutil"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/debug"
	"github.com/ernestrc/blue/issue"
	log "github.com/sirupsen/logrus"
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
	args, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	discard := log.New()
	discard.Out = ioutil.Discard

	ok, report := debug.CapturePanic(discard, "blue", s.version, func() {
		panic("this is a simulation")
	})
	if ok {
		log.Fatal("expected panic report")
	}
	return s.t.AddReport(ctx, report)
}
