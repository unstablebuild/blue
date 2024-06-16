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
package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/cli"
)

const defaultRegexpStr = `(?:\/\/.*(?:\r?\n|\r)?)+`

var defaultRegexp = regexp.MustCompile(defaultRegexpStr)

type licenseCli struct {
	fs        *cli.FlagSet
	regexpStr string
}

func newLicenseCli() *licenseCli {
	fs := cli.NewFlagSet("license")
	ret := &licenseCli{fs: fs}
	ret.fs.StringVar(&ret.regexpStr, "e", defaultRegexpStr,
		"Regexp to use to search and replace the license header")
	return ret
}

func (c *licenseCli) Man() cli.Manual {
	var cmds []cli.Manual
	return cli.Manual{
		Name:     "license",
		Summary:  "License files by either updating, or adding the license header found in <license>",
		Synopsis: "<license> <file> ...",
		Commands: cmds,
		Options:  *c.fs,
	}
}

func (c *licenseCli) Run(ctx context.Context, args []string) error {
	_, args, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	if len(args) < 2 {
		return cli.ErrInvalidArgs
	}

	var r *regexp.Regexp
	if c.regexpStr == defaultRegexpStr {
		r = defaultRegexp
	} else {
		r, err = regexp.Compile(c.regexpStr)
		if err != nil {
			return fmt.Errorf("compile regexp: %w", err)
		}
	}

	licenseFile := args[0]
	files := args[1:]

	license, err := readFile(licenseFile)
	if err != nil {
		return fmt.Errorf("license: %w", err)
	}

	results := make([]result, len(files))
	var wg sync.WaitGroup
	wg.Add(len(files))
	for i, filename := range files {
		go func(i int, license, filename string) {
			defer wg.Done()
			results[i] = processFile(ctx, r, license, filename)
		}(i, license, filename)
	}
	wg.Wait()

	var updated int
	var added int
	var already int
	var errors int
	for _, result := range results {
		if result.err != nil {
			err = multierror.Append(err, fmt.Errorf("%s: %v", result.filename, result.err))
			errors++
		} else if result.added {
			added++
		} else if result.updated {
			updated++
		} else {
			already++
		}
	}

	fmt.Fprintf(os.Stderr, "Ok: %d files, Updated: %d files, Added: %d files\n", already, updated, added)
	if errors != 0 {
		fmt.Fprintf(os.Stderr, "Errors: %d\n", errors)
	}

	return err
}

func readFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

type result struct {
	filename string
	updated  bool
	added    bool
	err      error
}

func processFile(
	ctx context.Context, r *regexp.Regexp, license, filename string,
) (ret result) {
	ret.filename = filename

	content, err := readFile(filename)
	if err != nil {
		ret.err = fmt.Errorf("read: %w", err)
		return
	}

	if strings.Contains(content, strings.TrimSpace(license)) {
		return
	}

	header := r.FindString(content)
	if containsLicenseHeader(header, content) {
		newContent := replaceHeader(content, header, license)
		err := os.WriteFile(filename, []byte(newContent), 0666)
		if err != nil {
			ret.err = fmt.Errorf("write: %w", err)
			return
		}
		ret.updated = true
		return
	}

	newContent := insertHeader(content, license)
	err = os.WriteFile(filename, []byte(newContent), 0666)
	if err != nil {
		ret.err = fmt.Errorf("write: %w", err)
		return
	}
	ret.added = true
	return
}

func containsLicenseHeader(header, content string) bool {
	header = strings.ToLower(header)
	containsCopyright := strings.Contains(header, "copyright")
	containsLicense := strings.Contains(header, "license")
	return containsCopyright || containsLicense
}

func insertHeader(content, header string) string {
	return strings.TrimSpace(header) + "\n" + strings.TrimLeft(content, "\n")
}

func replaceHeader(content, oldHeader, header string) (res string) {
	res = strings.ReplaceAll(content, strings.TrimSpace(oldHeader), strings.TrimSpace(header))
	return res
}
