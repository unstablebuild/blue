package secret

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ernestrc/sensible/editor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const authorKey = "author"

type secretData []byte

type manifest struct {
	ID        string
	Data      secretData
	Revoked   bool
	Metadata  map[string]string
	CreatedAt string `yaml:"created_at,omitempty"`
	UpdatedAt string `yaml:"updated_at,omitempty"`
}

func (m manifest) validate() error {
	if m.ID == "" {
		return errors.New("name cannot be empty")
	}
	if len(m.Data) == 0 {
		return errors.New("notes cannot be empty")
	}
	return nil
}

func (m manifest) toYAML() (string, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		err = fmt.Errorf("failed to encode stored manifest to yaml: %v", err)
		return "", err
	}
	return string(data), nil
}

func (d secretData) MarshalYAML() (interface{}, error) {
	return base64.StdEncoding.EncodeToString(d), nil
}

func (d *secretData) UnmarshalYAML(node *yaml.Node) error {
	value := node.Value
	ba, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return fmt.Errorf("decode base64: %v", err)
	}
	*d = ba
	return nil
}

func tempSecret(
	id string, dataFile string, author string, extraMdata map[string]string,
) (ret manifest, err error) {
	log.Debugf("decoding secret %q manifest from temp file with metadata: %#v",
		id, extraMdata)

	f, err := ioutil.TempFile("", "blue-secret")
	if err != nil {
		err = fmt.Errorf("failed create temp file: %v", err)
		return manifest{}, err
	}

	defer os.Remove(f.Name())

	mdata := map[string]string{
		authorKey: author,
	}

	for k, v := range extraMdata {
		mdata[k] = v
	}

	secretData, err := os.ReadFile(dataFile)
	if err != nil {
		return ret, fmt.Errorf("read data file: %w", err)
	}

	if len(secretData) == 0 {
		return ret, errors.New("empty data file")
	}

	// remove last EOL
	if secretData[len(secretData)-1] == '\n' {
		secretData = secretData[:len(secretData)-1]
	}

	m := manifest{ID: id, Data: secretData, Metadata: mdata}
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

	log.Debugf("decoded secret %q manifest from temp file: %#v",
		id, n)

	return n, n.validate()
}
