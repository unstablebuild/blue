package issue

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"

	"github.com/ernestrc/blue/issue"
	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

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

func tempIssue(ret issue.Report, defaultAuthor string) (issue.Report, error) {

	for {
		f, err := ioutil.TempFile("", "blue-issue")
		if err != nil {
			err = fmt.Errorf("failed create temp file: %v", err)
			return issue.Report{}, err
		}

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
