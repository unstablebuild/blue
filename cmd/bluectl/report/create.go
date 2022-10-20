package report

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/issue"
	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type issueCreate struct {
	t  issue.Tracker
	fs *cli.FlagSet
}

func tempIssue(defaultAuthor string) (issue.Report, error) {
	f, err := ioutil.TempFile("", "blue-issue")
	if err != nil {
		err = fmt.Errorf("failed create temp file: %v", err)
		return issue.Report{}, err
	}

	ret := issue.Report{
		Author: defaultAuthor,
	}

	for {
		dataIn, err := yaml.Marshal(&ret)
		if err != nil {
			err = fmt.Errorf("failed to marshal issue.Report into yaml: %v", err)
			return issue.Report{}, err
		}

		_, err = f.Write(dataIn)
		if err != nil {
			err = fmt.Errorf("failed write data to temp file: %v", err)
			return issue.Report{}, err
		}

		err = editor.Edit(f)
		if err != nil {
			err = fmt.Errorf("failed to edit manifest: %v", err)
			return issue.Report{}, err
		}

		err = f.Sync()
		if err != nil {
			err = fmt.Errorf("failed to sync temp file: %v", err)
			return issue.Report{}, err
		}
		err = f.Close()
		if err != nil {
			err = fmt.Errorf("failed to close temp file: %v", err)
			return issue.Report{}, err
		}

		data, err := ioutil.ReadFile(f.Name())
		if err != nil {
			err = fmt.Errorf("failed read data from temp file: %v", err)
			return issue.Report{}, err
		}

		ret = issue.Report{}
		err = yaml.Unmarshal(data, &ret)
		if err != nil {
			err = fmt.Errorf("failed decode yaml from temp file: %v", err)
			return issue.Report{}, err
		}

		log.Debugf("decoded issue manifest from temp file: %#v", ret)

		if ret.Package != "" && ret.Subject != "" && ret.Author != "" {
			break
		}
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Missing mandatory fields: package, subject and author. " +
			"Do you want to edit the issue? [y/n]:")
		text, _ := reader.ReadString('\n')
		switch text {
		case "y\n", "Y\n":
			continue
		default:
			return issue.Report{}, errors.New("could not submit issue")
		}
	}

	return ret, nil
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

func newReportCreateCLI(t issue.Tracker) cli.CLI {
	c := &issueCreate{
		t: t,
	}
	c.fs = cli.NewFlagSet("create")
	return c
}

func (s *issueCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create an issue in the issue tracker",
		Synopsis: "",
		Options:  *s.fs,
	}
}

func (s *issueCreate) Run(ctx context.Context, args []string) error {
	_, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	r, err := tempIssue(getDefaultAuthor())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	return s.t.AddReport(ctx, r)
}
