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

func tempIssue(ret issue.Report) (issue.Report, error) {
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
