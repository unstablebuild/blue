package release

import (
	"fmt"
	"io/ioutil"

	"github.com/ernestrc/blue/release"
	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const authorKey = "author"

// manifest represents the structure that the author of the release
// completes before pushing it.
type manifest struct {
	ID       string
	Notes    string
	Metadata map[string]string
}

func (m *manifest) fromModel(man release.Manifest) {
	m.ID = man.ID
	m.Notes = man.Notes
	m.Metadata = man.Metadata
}

func (m manifest) validate() error {
	// TODO do input validation + mandatory fields
	return nil
}

func (m manifest) toModel() release.Manifest {
	return release.Manifest{
		ID:       m.ID,
		Notes:    m.Notes,
		Metadata: m.Metadata,
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

func tempManifest(ID string, author string, extraMdata map[string]string) (ret release.Manifest, err error) {
	log.Debugf("decoding release %s manifest from temp file with metadata: %#v", ID, extraMdata)

	f, err := ioutil.TempFile("", "blue-release")
	if err != nil {
		err = fmt.Errorf("failed create temp file: %v", err)
		return release.Manifest{}, err
	}

	mdata := map[string]string{
		authorKey: author,
	}

	for k, v := range extraMdata {
		mdata[k] = v
	}

	m := manifest{ID: ID, Metadata: mdata}
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

	return m.toModel(), m.validate()
}
