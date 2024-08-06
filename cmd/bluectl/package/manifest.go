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
	"errors"
	"fmt"
	"os"

	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/release"
	"gopkg.in/yaml.v3"
)

const authorKey = "author"

// manifest represents the structure that the author of the package
// completes before uploading it.
type manifest struct {
	Name      string
	Notes     string
	Latest    string `yaml:"latest,omitempty"`
	Metadata  map[string]string
	CreatedAt string `yaml:"created_at,omitempty"`
}

func (m *manifest) fromModel(man release.Package) {
	m.Name = man.Name
	m.Notes = man.Notes
	m.Latest = string(man.Latest)
	m.Metadata = man.Metadata
	m.CreatedAt = man.CreatedAt.Format("2006-01-02T15:04:05.999Z")
}

func (m manifest) validate() error {
	if m.Name == "" {
		return errors.New("name cannot be empty")
	}
	if m.Notes == "" {
		return errors.New("notes cannot be empty")
	}
	return nil
}

func (m manifest) toModel() release.Package {
	return release.Package{
		Name:     m.Name,
		Notes:    m.Notes,
		Metadata: m.Metadata,
		// Latest and CreatedAt should never be populated by
		// the CLI
	}
}
func (m manifest) toYAML() (string, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		err = fmt.Errorf("failed to encode stored manifest to yaml: %v", err)
		return "", err
	}
	return string(data), nil
}

func tempPackage(
	pack string, author string, extraMdata map[string]string,
) (ret release.Package, err error) {
	log.Debugf("decoding package %q manifest from temp file with metadata: %#v",
		pack, extraMdata)

	f, err := os.CreateTemp("", "blue-release")
	if err != nil {
		err = fmt.Errorf("failed create temp file: %v", err)
		return release.Package{}, err
	}

	mdata := map[string]string{
		authorKey: author,
	}

	for k, v := range extraMdata {
		mdata[k] = v
	}

	m := manifest{Name: pack, Metadata: mdata}
	dataIn, err := yaml.Marshal(&m)
	if err != nil {
		panic(err)
	}

	_, err = f.Write(dataIn)
	if err != nil {
		err = fmt.Errorf("failed write data to temp file: %v", err)
		return
	}

	err = editor.Edit(f)
	if err != nil {
		err = fmt.Errorf("failed to edit manifest: %v", err)
		return
	}

	err = f.Sync()
	if err != nil {
		err = fmt.Errorf("failed to sync temp file: %v", err)
		return
	}
	err = f.Close()
	if err != nil {
		err = fmt.Errorf("failed to close temp file: %v", err)
		return
	}

	data, err := os.ReadFile(f.Name())
	if err != nil {
		err = fmt.Errorf("failed read data from temp file: %v", err)
		return
	}

	var n manifest
	err = yaml.Unmarshal(data, &n)
	if err != nil {
		err = fmt.Errorf("failed decode yaml from temp file: %v", err)
		return
	}

	log.Debugf("decoded package %q manifest from temp file: %#v",
		pack, n)

	return n.toModel(), n.validate()
}
