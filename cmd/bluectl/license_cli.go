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
	fs             *cli.FlagSet
	regexpStr      string
	errorOnChanges bool
	forceReplace   bool
}

func newLicenseCli() *licenseCli {
	fs := cli.NewFlagSet("license")
	ret := &licenseCli{fs: fs}
	ret.fs.StringVar(&ret.regexpStr, "e", defaultRegexpStr,
		"Regexp to use to search and replace the license header")
	ret.fs.BoolVar(&ret.forceReplace, "f", false,
		"Force update license header. If this flag is not passed, "+
			"then license headers are only added, not updated.")
	ret.fs.BoolVar(&ret.errorOnChanges, "d", false,
		"Exit with non-zero status instead of making changes if any "+
			"of the given files need to be updated.")
	return ret
}

func (c *licenseCli) Man() cli.Manual {
	var cmds []cli.Manual
	return cli.Manual{
		Name: "license",
		Summary: "License files by either updating, or adding the license " +
			"header found in the given license file.",
		Synopsis: "[options] <license> <file>...",
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
			results[i] = processFile(ctx, r, license, filename,
				c.errorOnChanges, c.forceReplace)
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

	if c.errorOnChanges {
		if added != 0 || updated != 0 {
			err = multierror.Append(err, fmt.Errorf("would have updated %d files "+
				"and added license header to %d files", updated, added))
		}
		return err
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
	ctx context.Context, r *regexp.Regexp,
	license, filename string, dryRun, forceReplace bool,
) (ret result) {
	ret.filename = filename

	content, err := readFile(filename)
	if err != nil {
		ret.err = fmt.Errorf("read: %w", err)
		return
	}

	if strings.Contains(content, strings.TrimSpace(license)+"\n\n") {
		return
	}

	header := r.FindString(content)
	if !containsLicenseHeader(header, content) {
		newContent := insertHeader(content, license)
		if dryRun {
			ret.added = true
			return
		}
		err = os.WriteFile(filename, []byte(newContent), 0666)
		if err != nil {
			ret.err = fmt.Errorf("write: %w", err)
			return
		}
		ret.added = true
		return
	}

	if !forceReplace {
		return
	}

	newContent := replaceHeader(content, header, license)
	if dryRun {
		ret.updated = true
		return
	}
	err = os.WriteFile(filename, []byte(newContent), 0666)
	if err != nil {
		ret.err = fmt.Errorf("write: %w", err)
		return
	}
	ret.updated = true
	return
}

func containsLicenseHeader(header, content string) bool {
	header = strings.ToLower(header)
	containsCopyright := strings.Contains(header, "copyright")
	containsLicense := strings.Contains(header, "license")
	return containsCopyright || containsLicense
}

func insertHeader(content, header string) string {
	return strings.TrimSpace(header) + "\n\n" + strings.TrimLeft(content, "\n")
}

// isDirective reports whether c is a comment directive.
// See go.dev/issue/37974.
// This code is also in go/ast.
//
// NOTE: copy-pasted from
// https://github.com/golang/go/blob/f428c7b729d3d9b37ed4dacddcd7ff88f4213f70/src/go/printer/comment.go#L110
// after reading it in
// https://github.com/golang/go/issues/43776#issuecomment-1159233421
func isDirective(c string) bool {
	// "//line " is a line directive.
	// "//extern " is for gccgo.
	// "//export " is for cgo.
	// (The // has been removed.)
	if strings.HasPrefix(c, "line ") ||
		strings.HasPrefix(c, "extern ") ||
		strings.HasPrefix(c, "export ") {
		return true
	}

	// "//[a-z0-9]+:[a-z0-9]"
	// (The // has been removed.)
	colon := strings.Index(c, ":")
	if colon <= 0 || colon+1 >= len(c) {
		return false
	}
	for i := 0; i <= colon+1; i++ {
		if i == colon {
			continue
		}
		b := c[i]
		if !('a' <= b && b <= 'z' || '0' <= b && b <= '9') {
			return false
		}
	}
	return true
}

func replaceHeader(content, oldHeader, header string) (res string) {
	// Preserve directives that might come glued after the header without a new
	// line, such as in:
	//
	//	1 // Copyright's first line bla bla bla bla
	//	2 // more literal text about license bla bla
	//	3 // ...
	//	4 // last line of license header file bla bla.
	//	5 //nolint:gosimple
	//	6 package coolpkg
	//
	//	We want to end up with:
	//
	//	1 // Copyright's first line bla bla bla bla
	//	2 // more literal text about license bla bla
	//	3 // ...
	//	4 // last line of license header file bla bla.
	//	5
	//	6 //nolint:gosimple
	//	7 package coolpkg
	//
	// NOTE: 1, 2, 3 line number indicators needed here otherwise to prevent
	// gofmt to remove the L5 on the block above.
	//
	preservedDirectives := ""

	for _, line := range strings.Split(oldHeader, "\n") {
		if strings.HasPrefix(line, "//") {
			directive := strings.TrimPrefix(line, "//")
			directive = strings.TrimSpace(directive)
			if isDirective(directive) {
				preservedDirectives += "\n" + line
			}
		}
	}

	res = strings.ReplaceAll(
		content,
		strings.TrimSpace(oldHeader),
		strings.TrimSpace(header)+"\n"+preservedDirectives,
	)
	return res
}
