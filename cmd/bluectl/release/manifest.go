package release

import (
	"fmt"
	"io/ioutil"

	"github.com/ernestrc/blue/release"
	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// manifest represents the structure that the author of the release
// completes before pushing it.
type manifest struct {
	ID     string
	Author string
	Notes  string
}

func (m manifest) toModel() release.Manifest {
	return release.Manifest{
		ID:     m.ID,
		Author: m.Author,
		Notes:  m.Notes,
	}
}

func tempManifest(ID string) (ret release.Manifest, err error) {
	f, err := ioutil.TempFile("", "blue-release")
	if err != nil {
		err = fmt.Errorf("failed create temp file: %v", err)
		return release.Manifest{}, err
	}

	m := manifest{ID: ID}
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
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		err = fmt.Errorf("failed to seek temp file: %v", err)
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		err = fmt.Errorf("failed read data from temp file: %v", err)
		return
	}

	err = yaml.Unmarshal(data, &m)
	if err != nil {
		err = fmt.Errorf("failed decode yaml from temp file: %v", err)
		return
	}

	log.Debugf("decoded release %s manifest from temp file: %#v", ID, m)

	return m.toModel(), nil
}
