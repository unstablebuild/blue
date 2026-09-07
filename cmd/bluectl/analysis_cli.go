// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"debug/elf"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/unstablebuild/blue/cli"
	"github.com/go-delve/delve/pkg/proc"
)

type analysisCli struct {
	fs  *cli.FlagSet
	csv bool
}

func newAnalysisCli() *analysisCli {
	fs := cli.NewFlagSet("analysis")
	ret := &analysisCli{fs: fs}

	ret.fs.BoolVar(&ret.csv, "c", false, "Print data in CSV format")
	return ret
}

func (c *analysisCli) Man() cli.Manual {
	var cmds []cli.Manual
	return cli.Manual{
		Name:     "analysis",
		Summary:  "Run Go linked package analysis against an executable",
		Synopsis: "[options] <path-to-executable>",
		Commands: cmds,
		Options:  *c.fs,
	}
}

func (c *analysisCli) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(c, c.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	executable := args[0]

	// Use delve to decode the DWARF section
	binInfo := proc.NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err = binInfo.AddImage(executable, 0)
	if err != nil {
		return fmt.Errorf("bin.AddImage: %s", err)
	}

	// Make a list of unique packages
	pkgs := make([]string, 0, len(binInfo.PackageMap))
	for _, fullPkgs := range binInfo.PackageMap {
		for _, fullPkg := range fullPkgs {
			exists := false
			for _, pkg := range pkgs {
				if fullPkg == pkg {
					exists = true
					break
				}
			}
			if !exists {
				pkgs = append(pkgs, fullPkg)
			}
		}
	}
	// Sort them for a nice output
	sort.Strings(pkgs)

	// Parse the ELF file ourselfs
	elfFile, err := elf.Open(executable)
	if err != nil {
		return fmt.Errorf("elf.Open: %v", err)
	}

	// Get the symbol table
	symbols, err := elfFile.Symbols()
	if err != nil {
		return fmt.Errorf("elf.Symbols: %v", err)
	}

	usage := make(map[string]map[string]int)

	for _, sym := range symbols {
		if sym.Section == elf.SHN_UNDEF || sym.Section >= elf.SectionIndex(len(elfFile.Sections)) {
			continue
		}

		sectionName := elfFile.Sections[sym.Section].Name

		symPkg := ""
		for _, pkg := range pkgs {
			if strings.HasPrefix(sym.Name, pkg) {
				symPkg = pkg
				break
			}
		}
		// Symbol doesn't belong to a known package
		if symPkg == "" {
			continue
		}

		pkgStats := usage[symPkg]
		if pkgStats == nil {
			pkgStats = make(map[string]int)
		}

		pkgStats[sectionName] += int(sym.Size)
		usage[symPkg] = pkgStats
	}

	if c.csv {
		fmt.Printf("package,bytes\n")
	}

	for _, pkg := range pkgs {
		sections, exists := usage[pkg]
		if !exists {
			continue
		}

		if c.csv {
			printCsv(pkg, sections)
		} else {
			printHuman(pkg, sections)
		}
	}

	return nil
}

func printHuman(pkg string, sections map[string]int) {
	fmt.Printf("%s:\n", pkg)
	for section, size := range sections {
		fmt.Printf("%15s: %8d bytes\n", section, size)
	}
	fmt.Println()
}

func printCsv(pkg string, sections map[string]int) {
	fmt.Printf("%s,", pkg)
	var total int
	for _, size := range sections {
		total += size
	}
	fmt.Printf("%d\n", total)
}
