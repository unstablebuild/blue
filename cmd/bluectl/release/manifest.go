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
package release

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

// manifest represents the structure that the author of the release
// completes before uploading it.
type manifest struct {
	Package   string
	Version   release.Version
	Notes     string
	Metadata  map[string]string
	CreatedAt string `yaml:"created_at,omitempty"`
}

func (m *manifest) fromModel(man release.Bundle) {
	m.Package = man.Package
	m.Version = man.Version
	m.Notes = man.Notes
	m.Metadata = man.Metadata
	m.CreatedAt = man.CreatedAt.Format("2006-01-02T15:04:05.999Z")
}

func (m manifest) validate() error {
	if m.Package == "" {
		return errors.New("package cannot be empty")
	}
	if m.Version == "" {
		return errors.New("version cannot be empty")
	}
	return nil
}

func (m manifest) toModel() release.Bundle {
	return release.Bundle{
		Package:  m.Package,
		Version:  m.Version,
		Notes:    m.Notes,
		Metadata: m.Metadata,
		// and CreatedAt should never be populated by the CLI
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

func tempBundle(
	pack string, ver release.Version,
	author string, extraMdata map[string]string,
	interactive bool,
) (ret release.Bundle, err error) {
	log.Debugf("decoding package %q release %q manifest from temp file with metadata: %#v",
		pack, ver, extraMdata)

	f, err := os.CreateTemp("", "blue-release")
	if err != nil {
		err = fmt.Errorf("failed create temp file: %v", err)
		return release.Bundle{}, err
	}

	mdata := map[string]string{
		authorKey: author,
	}

	for k, v := range extraMdata {
		mdata[k] = v
	}

	m := manifest{Package: pack, Version: ver, Metadata: mdata}
	dataIn, err := yaml.Marshal(&m)
	if err != nil {
		panic(err)
	}

	_, err = f.Write(dataIn)
	if err != nil {
		err = fmt.Errorf("failed write data to temp file: %v", err)
		return
	}

	if interactive {
		err = editor.Edit(f)
		if err != nil {
			err = fmt.Errorf("failed to edit manifest: %v", err)
			return
		}
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

	err = yaml.Unmarshal(data, &m)
	if err != nil {
		err = fmt.Errorf("failed decode yaml from temp file: %v", err)
		return
	}

	log.Debugf("decoded package %q release %q manifest from temp file: %#v",
		pack, ver, m)

	return m.toModel(), m.validate()
}
