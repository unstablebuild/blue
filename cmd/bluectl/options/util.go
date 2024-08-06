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
