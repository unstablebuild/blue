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

package issue

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/user"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/issue"
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

func tempIssue(ret issue.Report, interactive bool) (issue.Report, error) {
	f, err := os.CreateTemp("", "blue-issue")
	if err != nil {
		err = fmt.Errorf("create temp file: %v", err)
		return issue.Report{}, err
	}

	dataIn, err := yaml.Marshal(&ret)
	if err != nil {
		err = fmt.Errorf("marshal issue.Report into yaml: %v", err)
		return issue.Report{}, err
	}

	_, err = f.Write(dataIn)
	if err != nil {
		err = fmt.Errorf("write data to temp file: %v", err)
		return issue.Report{}, err
	}

	err = f.Sync()
	if err != nil {
		err = fmt.Errorf("sync data to temp file: %v", err)
		return issue.Report{}, err
	}

	// reset report so deleted fields are reset, for instance
	// in case this is an edit
	ret = issue.Report{}

	if !interactive {
		data, err := os.ReadFile(f.Name())
		if err != nil {
			return issue.Report{}, fmt.Errorf("read data from temp file: %v", err)
		}
		if err := yaml.Unmarshal(data, &ret); err != nil {
			return issue.Report{}, fmt.Errorf("unmarshal yaml from temp file: %v", err)
		}
		if ret.Package == "" || ret.Subject == "" || ret.Author == "" {
			return issue.Report{}, fmt.Errorf("missing one or more mandatory fields: " +
				"package, subject or author")
		}
		_ = f.Close()
		return ret, nil
	}

	for {
		err = editor.Edit(f)
		if err != nil {
			err = fmt.Errorf("edit manifest: %v", err)
			return issue.Report{}, err
		}

		data, err := os.ReadFile(f.Name())
		if err != nil {
			err = fmt.Errorf("read data from temp file: %v", err)
			return issue.Report{}, err
		}

		if string(data) == string(dataIn) {
			err = fmt.Errorf("issue was not modified. ")
		} else {
			err = yaml.Unmarshal(data, &ret)
			if err != nil {
				err = fmt.Errorf("unmarshal yaml from temp file: %v", err)
			} else {
				log.Debugf("decoded issue manifest from temp file: %#v", ret)
				if ret.Package == "" || ret.Subject == "" || ret.Author == "" {
					err = multierr.Append(err, fmt.Errorf("missing one or more manatory fields: "+
						"package, subject or author"))
				}
			}
		}

		if err == nil {
			break
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Println(err.Error() +
			"Do you want to edit the issue? [y/n]:")
		text, _ := reader.ReadString('\n')
		switch text {
		case "y\n", "Y\n":
			continue
		default:
			return issue.Report{}, errors.New("did not submit issue")
		}
	}

	err = f.Close()
	if err != nil {
		err = fmt.Errorf("close temp file: %v", err)
		return issue.Report{}, err
	}
	return ret, nil
}
