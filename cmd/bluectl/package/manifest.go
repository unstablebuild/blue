package pack

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/ernestrc/blue/release"
	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const authorKey = "author"

// manifest represents the structure that the author of the package
// completes before uploading it.
type manifest struct {
	Name     string
	Notes    string
	Metadata map[string]string
}

func (m *manifest) fromModel(man release.Package) {
	m.Name = man.Name
	m.Notes = man.Notes
	m.Metadata = man.Metadata
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

	f, err := ioutil.TempFile("", "blue-release")
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
	}
	err = f.Close()
	if err != nil {
		err = fmt.Errorf("failed to close temp file: %v", err)
	}

	data, err := ioutil.ReadFile(f.Name())
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
