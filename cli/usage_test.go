package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

const expectedGitAddUsage = `Add file contents to the index

Usage: add [options]

Options: 
  -h        Display this message [default: false]
  -version  display cli version [default: false]

`

const expectedGitUsage = `Git is the stupid content tracker

Usage: git [options] <cmd>

Options: 
  -h        Display this message [default: false]
  -version  display cli version [default: false]

Commands: 
  commit    Record changes to the repository
  add       Add file contents to the index

`

const expectedEmptyUsage = "Usage:  \n\nOptions: \n  -h  Display this message [default: false]\n\n"

func TestUsage(t *testing.T) {

	tsuite := []struct {
		in  testCLI
		out string
	}{
		{emptyManual, expectedEmptyUsage},
		{gitManual, expectedGitUsage},
		{gitAddManual, expectedGitAddUsage},
	}

	for _, tcase := range tsuite {
		var b bytes.Buffer
		tcase.in.options.SetOutput(&b)

		Usage(tcase.in)

		assert.Equal(t, tcase.out, b.String())
	}
}
