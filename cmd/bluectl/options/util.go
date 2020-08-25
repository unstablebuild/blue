package options

import (
	"fmt"
	"os"
)

// PathExists returns whether path exists and it if it's a directory.
func PathExists(path string) (exists bool, isDir bool, err error) {
	var info os.FileInfo
	info, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
			return
		}
		return
	}
	isDir = info.IsDir()
	exists = true
	return
}

// OpenFile verifies that path is a standard file and opens it.
func OpenFile(path string) (f *os.File, err error) {
	exists, isDir, err := PathExists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("file not found '%s'", path)
	}
	if isDir {
		return nil, fmt.Errorf(
			"expected '%s' to be a standard file but it's a directory", path)
	}

	f, err = os.Open(path)
	return
}

// EnsureFolderExists checks for the existence of folder. If it doesn't exist
// it will create it. If it's not a dir, it will return an error.
func EnsureFolderExists(folder string) error {
	ok, isDir, err := PathExists(folder)
	if err != nil {
		return err
	}
	if !ok {
		err = os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			return err
		}
		isDir = true
	}

	if !isDir {
		err = fmt.Errorf("config directory %s is not a directory", folder)
		return err
	}

	return nil
}
