// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package idepkg

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/ide/idepkg/idepkgtest"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/walkdir"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"gopkg.in/yaml.v3"
)

func TestLibDir(t *testing.T) {
	t.Parallel()
	t.Run("returns installed files", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		it, err := m.LibDir(context.Background(), "go")
		require.NoError(t, err)
		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		// test that paths are absolute
		actual := make([]string, 0)
		for _, path := range installed {
			require.True(t, filepath.IsAbs(path))
			actual = append(actual, filepath.Base(path))
		}
		expected := []string{
			"highlights.scm",
			"tags.scm",
			"indents.scm",
			"tree-sitter.so",
			"go",
			"gofmt",
			"goimports",
			"gopls",
		}
		assert.ElementsMatch(t, expected, actual)
	})
	t.Run("if install started, returned iterator blocks until package is done installing", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)

		it, err := m.LibDir(context.Background(), "go")
		require.NoError(t, err)
		actual, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.NotEmpty(t, actual)
	})
	t.Run("multiple cals to LibDir while installing return complete iterators", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)

		it1, err := m.LibDir(context.Background(), "go")
		require.NoError(t, err)

		it2, err := m.LibDir(context.Background(), "go")
		require.NoError(t, err)

		it3, err := m.LibDir(context.Background(), "go")
		require.NoError(t, err)

		for _, it := range []iterator.Iterator[string]{it1, it2, it3} {
			files, err := iterator.ToSlice(context.Background(), it)
			require.NoError(t, err)
			var actual []string
			for _, file := range files {
				actual = append(actual, filepath.Base(file))
			}
			expected := []string{
				"highlights.scm", "tags.scm", "indents.scm",
				"tree-sitter.so", "go",
				"gofmt", "goimports", "gopls",
			}
			assert.ElementsMatch(t, expected, actual)
		}
	})
	t.Run("returns error if package is not installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.LibDir(context.Background(), "go")
		require.Equal(t, ErrNotInstalled, err)
	})
}

func TestDescribePackage(t *testing.T) {
	t.Parallel()
	t.Run("no packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.DescribePackage(context.Background(), "myPkg")
		assert.EqualError(t, err, "not found")
	})
	t.Run("returns error if package id is empty", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.DescribePackage(context.Background(), "")
		require.Error(t, err)
	})
	t.Run("returns package", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		pkg, err := m.DescribePackage(context.Background(), "go")
		require.NoError(t, err)
		assert.Equal(t, release.Package{Name: "go"}, pkg)
	})
	t.Run("bubbles up release manager error", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles()
		m, _, r, _ := newTestManager(t, pkgs, versions)

		r.ExpectReturnErr(errors.New("boom"))
		_, err := m.DescribePackage(context.Background(), "go")
		assert.EqualError(t, err, "boom")
	})
}

func TestDescribeRelease(t *testing.T) {
	t.Parallel()
	t.Run("no releases", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.DescribeRelease(context.Background(), "myPkg", "notFoundRelease")
		assert.EqualError(t, err, "not found")
	})
	t.Run("returns error if package id is empty", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "", Version: "m"}})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.DescribeRelease(context.Background(), "", "m")
		require.Error(t, err)
	})
	t.Run("returns error if release id is empty", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: ""}})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.DescribeRelease(context.Background(), "go", "")
		require.Error(t, err)
	})
	t.Run("returns release bundle", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "m"}})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		pkg, err := m.DescribeRelease(context.Background(), "go", "m")
		require.NoError(t, err)
		assert.Equal(t, release.Bundle{Package: "go", Version: "m"}, pkg)
	})
	t.Run("bubbles up release manager error", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "m"}})
		m, _, r, _ := newTestManager(t, pkgs, versions)

		r.ExpectReturnErr(errors.New("boom"))
		_, err := m.DescribeRelease(context.Background(), "go", "m")
		assert.EqualError(t, err, "boom")
	})
}

func TestListPackages(t *testing.T) {
	t.Parallel()
	t.Run("no packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		it, err := m.ListPackages(context.Background(), nil)
		require.NoError(t, err)

		_, empty := iterator.IsEmpty(context.Background(), it)
		assert.True(t, empty)
	})
	t.Run("returns packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"}, release.Package{Name: "ox"})
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		it, err := m.ListPackages(context.Background(), nil)
		require.NoError(t, err)

		actual, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.ElementsMatch(t, []release.Package{{Name: "go"}, {Name: "ox"}}, actual)
	})
	t.Run("bubbles up release manager error", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles()
		m, _, r, _ := newTestManager(t, pkgs, versions)

		r.ExpectReturnErr(errors.New("boom"))
		_, err := m.ListPackages(context.Background(), nil)
		assert.EqualError(t, err, "boom")
	})
}

func TestListPackageVersions(t *testing.T) {
	t.Parallel()
	t.Run("no packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.ListPackageVersions(context.Background(), "go", nil)
		require.Error(t, err)
		assert.EqualError(t, err, "not found")
	})
	t.Run("returns error if package id is empty", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.ListPackageVersions(context.Background(), "", nil)
		require.Error(t, err)
	})
	t.Run("returns packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		it, err := m.ListPackageVersions(context.Background(), "go", nil)
		require.NoError(t, err)

		actual, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.ElementsMatch(t, []release.Bundle{{Package: "go", Version: "1"}}, actual)
	})
	t.Run("bubbles up release manager error", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go"})
		versions := idepkgtest.MakeBundles()
		m, _, r, _ := newTestManager(t, pkgs, versions)

		r.ExpectReturnErr(errors.New("boom"))
		_, err := m.ListPackageVersions(context.Background(), "go", nil)
		assert.EqualError(t, err, "boom")
	})
}

func TestInstallPackageVersion(t *testing.T) {
	t.Parallel()
	t.Run("returns error if pkg is empty", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)
		err := m.InstallPackageVersion(context.Background(), "", "1")
		assert.Error(t, err)
	})

	t.Run("returns error if version is empty", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, _, _, _ := newTestManager(t, pkgs, versions)
		err := m.InstallPackageVersion(context.Background(), "go", "")
		assert.Error(t, err)
	})

	t.Run("installs a package with executables", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})

	t.Run("install a package and version already installed fails", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		err = m.InstallPackageVersion(context.Background(), "go", "1")
		assert.Error(t, err)
	})

	t.Run("installs a package without executables", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "testpkg", Version: "1"}})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "testpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		assertDataDirExists(t, datadir, "testpkg")
		assertExecutables(t, datadir)
	})

	t.Run("install errors are retryable", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, r, datadir := newTestManager(t, pkgs, versions)

		r.ExpectReturnErr(errors.New("boom"))
		n.ExpectErrorNotification = true
		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireErrorNotification()

		r.ExpectReturnErr(nil)
		n.Reset()
		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})
	t.Run("notification progress is completed, even if writer doesn't", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, r, datadir := newTestManager(t, pkgs, versions)
		r.SetMissProgressComplete(true)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()
		assert.Len(t, n.Active(), 1)

		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})
}

func TestListInstalledPackageVersions(t *testing.T) {
	t.Parallel()
	t.Run("returns installed packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		it, err := m.ListInstalledPackages(context.Background())
		require.NoError(t, err)

		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{"go"}, installed)
	})
	t.Run("deduplicates when multiple versions are installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		it, err := m.ListInstalledPackages(context.Background())
		require.NoError(t, err)

		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		assert.Equal(t, []string{"go"}, installed)
	})
	t.Run("returns nothing if there are no packages installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		it, err := m.ListInstalledPackages(context.Background())
		require.NoError(t, err)

		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Empty(t, installed)
	})
}

func TestListInstalledPackages(t *testing.T) {
	t.Parallel()
	t.Run("returns installed packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		it, err := m.ListInstalledPackageVersions(context.Background(), "go")
		require.NoError(t, err)

		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		assert.ElementsMatch(t, []release.Version{"1"}, installed)
	})
	t.Run("returns nothing if there are no packages installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _ := newTestManager(t, pkgs, versions)

		it, err := m.ListInstalledPackageVersions(context.Background(), "go")
		require.NoError(t, err)

		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Empty(t, installed)
	})
}

func TestPackageVersionInUse(t *testing.T) {
	t.Parallel()
	t.Run("returns installed package version in use", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		actual, err := m.PackageVersionInUse(context.Background(), "go")
		require.NoError(t, err)

		assert.Equal(t, release.Version("1"), actual)
	})

	t.Run("returns latest installed package version", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		actual, err := m.PackageVersionInUse(context.Background(), "go")
		require.NoError(t, err)

		assert.Equal(t, release.Version("2"), actual)
	})

	t.Run("returns package version in use, after UsePackageVersion", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		err = m.UsePackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)

		actual, err := m.PackageVersionInUse(context.Background(), "go")
		require.NoError(t, err)

		assert.Equal(t, release.Version("1"), actual)
	})

	t.Run("returns error if package is not installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		_, err := m.PackageVersionInUse(context.Background(), "go")
		require.Error(t, err)
	})
}

func TestUsePackageVersion(t *testing.T) {
	t.Parallel()
	t.Run("returns error if package is not installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, _, _, _ := newTestManager(t, pkgs, versions)

		err := m.UsePackageVersion(context.Background(), "go", "1")
		require.Error(t, err)
	})

	t.Run("returns error if version is not installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		err = m.UsePackageVersion(context.Background(), "go", "2")
		require.Error(t, err)
	})

	t.Run("relinks executables and lib", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		require.NoError(t, os.RemoveAll(filepath.Join(datadir, "bin", "go")))
		require.NoError(t, os.RemoveAll(filepath.Join(datadir, "lib", "go")))

		err = m.UsePackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)

		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})
	t.Run("returns error if version already in use", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		err = m.UsePackageVersion(context.Background(), "go", "2")
		require.Equal(t, err, ErrVersionInUse)
	})
}

func TestDeletePackage(t *testing.T) {
	t.Parallel()
	t.Run("deletes all versions of an installed package", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		err = m.DeletePackage(context.Background(), "go")
		require.NoError(t, err)

		assertDataDirNotExists(t, datadir, "go")
		assertExecutables(t, datadir /* none */)
	})

	t.Run("returns ErrNotInstalled if package is not installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		err = m.DeletePackage(context.Background(), "testpkg")
		require.Equal(t, ErrNotInstalled, err)
	})
}

func TestDeletePackageVersion(t *testing.T) {
	t.Parallel()
	t.Run("deletes a version of an installed package", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		err = m.DeletePackageVersion(context.Background(), "go", "1", false)
		require.NoError(t, err)

		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})
	t.Run("returns ErrNotInstalled if package and version is not installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		err = m.DeletePackageVersion(context.Background(), "go", "3", false)
		require.Equal(t, ErrNotInstalled, err)

		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})
	t.Run("returns ErrVersionInUse if package version is in use and force is false", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		err = m.DeletePackageVersion(context.Background(), "go", "2", false)
		require.Equal(t, err, ErrVersionInUse)

		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})
	t.Run("removes lib+executables if package version is in use and force is true", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wait()

		err = m.DeletePackageVersion(context.Background(), "go", "2", true)
		require.NoError(t, err)

		assertExecutables(t, datadir /* none */)

		// use package version remaining version works after delete+force
		err = m.UsePackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		assertDataDirExists(t, datadir, "go")
		assertExecutables(t, datadir, goTarExpectedExecutables...)
	})
}

func assertDataDirExists(t *testing.T, datadir string, pkgs ...string) {
	t.Helper()

	info, err := os.Stat(datadir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	info, err = os.Stat(filepath.Join(datadir, "bin"))
	require.NoError(t, err)
	require.True(t, info.IsDir())

	info, err = os.Stat(filepath.Join(datadir, "lib"))
	require.NoError(t, err)
	require.True(t, info.IsDir())

	info, err = os.Stat(filepath.Join(datadir, "pkg"))
	require.NoError(t, err)
	require.True(t, info.IsDir())

	files := listFiles(t, datadir)
	require.NoError(t, err)
	filesDebug := fmt.Sprintf("all files: %+v", files)

	for _, pkg := range pkgs {
		info, err = os.Stat(filepath.Join(datadir, "lib", pkg))
		require.NoError(t, err)
		assert.True(t, info.IsDir(), filesDebug)

		info, err = os.Stat(filepath.Join(datadir, "pkg", pkg))
		require.NoError(t, err)
		assert.True(t, info.IsDir(), filesDebug)
	}
}

func assertDataDirNotExists(t *testing.T, datadir string, pkgs ...string) {
	t.Helper()

	info, err := os.Stat(datadir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	for _, pkg := range pkgs {
		_, err = os.Stat(filepath.Join(datadir, "lib", pkg))
		require.Error(t, err)

		_, err = os.Stat(filepath.Join(datadir, "pkg", pkg))
		require.Error(t, err)
	}
}

var goTarExpectedExecutables = []string{
	"go", "gofmt", "goimports", "gopls", "tree-sitter.so",
}

func assertExecutables(t *testing.T, datadir string, expected ...string) {
	t.Helper()

	bindir := filepath.Join(datadir, "bin")
	info, err := os.Stat(bindir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	actual := listFiles(t, bindir)
	require.NoError(t, err)

	require.Len(t, actual, len(expected))
	assert.ElementsMatch(t, expected, actual)
}

func listFiles(t *testing.T, bindir string) []string {
	scheme := newLocalScheme(bindir)
	it, err := walkdir.ListFiles(context.Background(), scheme, ".")
	require.NoError(t, err)
	files, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	return files
}

var syncTick = func(fn func()) bool { fn(); return true }

func newTestManager(
	t *testing.T,
	packages map[string]release.Package,
	versions map[string][]release.Bundle,
) (*Manager, *idepkgtest.Notifications, *idepkgtest.ReleaseManager, string) {
	temp, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(temp)
	})
	configPath := filepath.Join(temp, "config.yaml")
	n := idepkgtest.NewNotifications(t)
	m := idepkgtest.NewReleaseManager(packages, versions)
	fileScheme := newLocalScheme(temp)
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			// Auto-accept: send Enter to select "Allow"
			h.Resize(70, 20)
			h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
			return &mockWindow{}, nil
		},
	}
	manager := NewManager(n, m, document.NewInMemoryService(),
		fileScheme, temp, configPath, wm, syncTick, term.NopInterrupter())
	return manager, n, m, temp
}

type mockWindowManager struct {
	floatingFn func(browserapi.Floating, browserapi.FloatingConfig) (browserapi.Window, error)
}

func (m *mockWindowManager) Focus() (browserapi.Window, error)        { return nil, nil }
func (m *mockWindowManager) Split(_ browserapi.Orientation, _ browserapi.Window, _ browserapi.Handler) (browserapi.Window, error) {
	return nil, nil
}
func (m *mockWindowManager) Floating(h browserapi.Floating, cfg browserapi.FloatingConfig) (browserapi.Window, error) {
	if m.floatingFn != nil {
		return m.floatingFn(h, cfg)
	}
	return &mockWindow{}, nil
}
func (m *mockWindowManager) Bar(_ browserapi.BarConfig, _ tui.Handler) error { return nil }
func (m *mockWindowManager) Tab(_ workspaceapi.URI, _ rune, _ string, _ browserapi.Handler) (browserapi.Handler, error) {
	return nil, nil
}
func (m *mockWindowManager) SetWindowContent(_ browserapi.Window, _ browserapi.Handler) error {
	return nil
}
func (m *mockWindowManager) CloseWindow(_ browserapi.Window) error { return nil }

type mockWindow struct{}

func (m *mockWindow) WindowID() uint64 { return 0 }

// localScheme implements schemeapi.Scheme using os package functions for testing.
type localScheme struct {
	root string
}

func newLocalScheme(root string) *localScheme {
	return &localScheme{root: root}
}

func (s *localScheme) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.root, path)
}

func (s *localScheme) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + s.resolve(path))
}

func (s *localScheme) Root() string { return s.root }

func (s *localScheme) NewFile(_ uintptr, _ string) workspaceapi.File {
	panic("not implemented")
}

func (s *localScheme) Chroot(path string) (schemeapi.Scheme, error) {
	return newLocalScheme(s.resolve(path)), nil
}

func (s *localScheme) Watch(_ string, _ chan<- schemeapi.EventInfo, _ ...schemeapi.Event) (int, error) {
	panic("not implemented")
}

func (s *localScheme) StopWatch(_ int) error {
	panic("not implemented")
}

// schemeapi.FileSystem

func (s *localScheme) Create(filename string) (workspaceapi.File, error) {
	return os.Create(s.resolve(filename))
}

func (s *localScheme) Open(filename string) (workspaceapi.File, error) {
	return os.Open(s.resolve(filename))
}

func (s *localScheme) OpenFile(filename string, flag int, perm fs.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(s.resolve(filename), flag, perm)
}

func (s *localScheme) Stat(filename string) (fs.FileInfo, error) {
	return os.Stat(s.resolve(filename))
}

func (s *localScheme) Rename(oldpath, newpath string) error {
	return os.Rename(s.resolve(oldpath), s.resolve(newpath))
}

func (s *localScheme) Remove(filename string) error {
	return os.Remove(s.resolve(filename))
}

func (s *localScheme) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (s *localScheme) TempFile(dir, prefix string) (workspaceapi.File, error) {
	return os.CreateTemp(s.resolve(dir), prefix)
}

func (s *localScheme) Lstat(filename string) (fs.FileInfo, error) {
	return os.Lstat(s.resolve(filename))
}

func (s *localScheme) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, s.resolve(newname))
}

func (s *localScheme) Readlink(link string) (string, error) {
	return os.Readlink(s.resolve(link))
}

func (s *localScheme) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(s.resolve(path))
}

func (s *localScheme) MkdirAll(filename string, perm fs.FileMode) error {
	return os.MkdirAll(s.resolve(filename), perm)
}

// schemeapi.Executor

func (s *localScheme) StartCommand(_ context.Context, _ workspaceapi.Cmd) (workspaceapi.Pid, error) {
	panic("not implemented")
}

func (s *localScheme) Signal(_ workspaceapi.Pid, _ syscall.Signal) error {
	panic("not implemented")
}

func (s *localScheme) Close() error { return nil }

// schemeapi.Terminal

func (s *localScheme) NewPty(_ context.Context) (workspaceapi.Pty, error) {
	panic("not implemented")
}

func (s *localScheme) SetPtySize(_ workspaceapi.Pty, _, _ int) error {
	panic("not implemented")
}

func readUserConfig(t *testing.T, datadir string) *yaml.Node {
	t.Helper()
	configPath := filepath.Join(datadir, "config.yaml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &doc))
	require.Equal(t, yaml.DocumentNode, doc.Kind)
	return &doc
}

func assertYAMLKey(t *testing.T, mapping *yaml.Node, key, expected string) {
	t.Helper()
	idx := findMappingKey(mapping, key)
	require.GreaterOrEqual(t, idx, 0, "key %q not found", key)
	assert.Equal(t, expected, mapping.Content[idx+1].Value)
}

func assertNestedYAMLKey(t *testing.T, mapping *yaml.Node, outerKey, innerKey, expected string) {
	t.Helper()
	idx := findMappingKey(mapping, outerKey)
	require.GreaterOrEqual(t, idx, 0, "outer key %q not found", outerKey)
	inner := mapping.Content[idx+1]
	require.Equal(t, yaml.MappingNode, inner.Kind, "outer key %q is not a mapping", outerKey)
	assertYAMLKey(t, inner, innerKey, expected)
}

func TestInstallPackageVersionConfig(t *testing.T) {
	t.Parallel()
	t.Run("config.yaml is merged into user config after install", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "configpkg", Version: "1"}})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		// Create an existing user config so processConfig merges into it
		configPath := filepath.Join(datadir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0644))

		n.SetWg(2) // apply success + download success
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		doc := readUserConfig(t, datadir)
		root := doc.Content[0]
		assertNestedYAMLKey(t, root, "env", "GOROOT",
			datadir+"/pkg/configpkg/1/go")
		assertNestedYAMLKey(t, root, "settings", "theme", "dark")
		assertNestedYAMLKey(t, root, "settings", "indent", "4")
	})
	t.Run("skipped when no user config exists", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "configpkg", Version: "1"}})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Config should not have been created
		_, err = os.Stat(filepath.Join(datadir, "config.yaml"))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestUsePackageVersionConfig(t *testing.T) {
	t.Parallel()
	t.Run("switching version updates user config", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "configpkg", Version: "1"},
			{Package: "configpkg", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		// Create an existing user config so processConfig merges into it
		configPath := filepath.Join(datadir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0644))

		n.SetWg(2) // apply success + download success
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		n.SetWg(2) // apply success + download success
		err = m.InstallPackageVersion(context.Background(), "configpkg", "2")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Prevent UsePackageVersion's synchronous prompt notification
		// from calling Done on a completed WaitGroup.
		n.ClearWg()

		// Switch to version 1
		require.NoError(t, os.RemoveAll(filepath.Join(datadir, "lib", "configpkg")))
		err = m.UsePackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)

		doc := readUserConfig(t, datadir)
		root := doc.Content[0]
		assertNestedYAMLKey(t, root, "env", "GOROOT",
			datadir+"/pkg/configpkg/1/go")
		assertNestedYAMLKey(t, root, "settings", "theme", "dark")
		assertNestedYAMLKey(t, root, "settings", "indent", "4")

		// Backup should exist since the config was modified
		_, err = os.Stat(filepath.Join(datadir, "config.yaml.backup"))
		require.NoError(t, err)
	})
}

func TestProcessInstalledSettingsConfig(t *testing.T) {
	t.Parallel()
	t.Run("merges config.yaml from installed packages into user config", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "configpkg", Version: "1"}})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		// Create an existing user config so processConfig merges into it
		configPath := filepath.Join(datadir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0644))

		n.SetWg(2) // apply success + download success
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Reset the user config to an empty mapping to force re-merge
		require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0644))
		_ = os.Remove(filepath.Join(datadir, "config.yaml.backup"))

		// Prevent ProcessInstalledSettings' synchronous prompt notification
		// from calling Done on a completed WaitGroup.
		n.ClearWg()

		err = m.ProcessInstalledSettings(context.Background())
		require.NoError(t, err)

		doc := readUserConfig(t, datadir)
		root := doc.Content[0]
		assertNestedYAMLKey(t, root, "env", "GOROOT",
			datadir+"/pkg/configpkg/1/go")
		assertNestedYAMLKey(t, root, "settings", "theme", "dark")
	})
	t.Run("skipped when no user config exists", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "configpkg", Version: "1"}})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()

		err = m.ProcessInstalledSettings(context.Background())
		require.NoError(t, err)

		// Config should not have been created
		_, err = os.Stat(filepath.Join(datadir, "config.yaml"))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestPromptConfigChangeRender(t *testing.T) {
	t.Parallel()
	prompt := handler.NewPrompt(handler.PromptConfig{
		PromptConfig: component.PromptConfig{
			Message: "Extension testpkg (v1) wants to update your configuration " +
				"with the following settings:\n\nenv:\n  GOROOT: /data/go\n\n" +
				"Do you want to allow this?",
			Options: []string{"Allow", "Deny"},
		},
		PromptHandler: handler.FuncPromptHandler(
			func(_ int, _ string) {},
			func() error { return nil },
		),
	})
	prompt.Resize(40, 15)
	rendered := handlertest.DrawHandler(prompt, 40, 15)
	assert.Contains(t, rendered, "Extension testpkg")
	assert.Contains(t, rendered, "Allow")
	assert.Contains(t, rendered, "Deny")
}

func TestPromptConfigChangeAllow(t *testing.T) {
	t.Parallel()
	var selected = -1
	prompt := handler.NewPrompt(handler.PromptConfig{
		PromptConfig: component.PromptConfig{
			Message: "Allow changes?",
			Options: []string{"Allow", "Deny"},
		},
		PromptHandler: handler.FuncPromptHandler(
			func(idx int, _ string) { selected = idx },
			func() error { return nil },
		),
	})
	prompt.Resize(40, 15)
	exit, handled := prompt.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, exit)
	assert.True(t, handled)
	assert.Equal(t, 0, selected)
}

func TestPromptConfigChangeDeny(t *testing.T) {
	t.Parallel()
	var selected = -1
	prompt := handler.NewPrompt(handler.PromptConfig{
		PromptConfig: component.PromptConfig{
			Message: "Allow changes?",
			Options: []string{"Allow", "Deny"},
		},
		PromptHandler: handler.FuncPromptHandler(
			func(idx int, _ string) { selected = idx },
			func() error { return nil },
		),
	})
	prompt.Resize(40, 15)
	// Move right to "Deny" then press Enter
	prompt.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
	exit, handled := prompt.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, exit)
	assert.True(t, handled)
	assert.Equal(t, 1, selected)
}

func TestPromptConfigChangeEscape(t *testing.T) {
	t.Parallel()
	var selected = -1
	prompt := handler.NewPrompt(handler.PromptConfig{
		PromptConfig: component.PromptConfig{
			Message: "Allow changes?",
			Options: []string{"Allow", "Deny"},
		},
		PromptHandler: handler.FuncPromptHandler(
			func(idx int, _ string) { selected = idx },
			func() error { return nil },
		),
	})
	prompt.Resize(40, 15)
	exit, handled := prompt.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	assert.True(t, exit)
	assert.True(t, handled)
	// OnSelect should not have been called
	assert.Equal(t, -1, selected)
}

func TestInstallConfigPromptDeny(t *testing.T) {
	t.Parallel()
	t.Run("prompt denies config merge, existing config unchanged", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "configpkg", Version: "1"},
			{Package: "configpkg", Version: "2"},
		})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		// Create an existing user config so the prompt path is taken
		configPath := filepath.Join(datadir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("existing: true\n"), 0644))

		// First install merges into existing config (auto-accepted by default mock)
		n.SetWg(2) // apply success + download success
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Capture config state after first install
		origConfig, err := os.ReadFile(configPath)
		require.NoError(t, err)

		// Override wm to simulate "Deny" (select index 1)
		m.wm = &mockWindowManager{
			floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
				h.Resize(70, 20)
				// Move to "Deny" then press Enter
				h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
				h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
				return &mockWindow{}, nil
			},
		}

		// Second install (v2 has different config values) — deny the prompt
		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "configpkg", "2")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Config should remain unchanged after deny
		afterConfig, err := os.ReadFile(configPath)
		require.NoError(t, err)
		assert.Equal(t, string(origConfig), string(afterConfig),
			"config.yaml should not change when prompt is denied")
	})
}

// --- Test helpers for crash-safe download tests ---

func createStaleEntry(
	t *testing.T, s document.Service, pkgID string, version release.Version,
) {
	t.Helper()
	key := fmt.Sprintf("%s:%s", pkgID, version)
	err := s.Create(context.Background(), key, pkgVersionValue{
		Package: pkgID, Version: version,
	})
	require.NoError(t, err)
}

func createCompleteEntry(
	t *testing.T, s document.Service, pkgID string, version release.Version,
) {
	t.Helper()
	key := fmt.Sprintf("%s:%s", pkgID, version)
	err := s.Create(context.Background(), key, pkgVersionValue{
		Package: pkgID, Version: version, Complete: true,
	})
	require.NoError(t, err)
}

func assertStorageEntryComplete(
	t *testing.T, s document.Service, pkgID string, version release.Version,
) {
	t.Helper()
	key := fmt.Sprintf("%s:%s", pkgID, version)
	var val pkgVersionValue
	err := s.Get(context.Background(), key, &val)
	require.NoError(t, err, "storage entry should exist")
	assert.True(t, val.Complete, "storage entry should be complete")
}

func assertStorageEntryNotExists(
	t *testing.T, s document.Service, pkgID string, version release.Version,
) {
	t.Helper()
	key := fmt.Sprintf("%s:%s", pkgID, version)
	var val pkgVersionValue
	err := s.Get(context.Background(), key, &val)
	assert.True(t, errors.Is(err, document.ErrNotFound),
		"storage entry should not exist, got: %v", err)
}

// newTestManagerWithStorage is like newTestManager but returns the storage service too.
func newTestManagerWithStorage(
	t *testing.T,
	packages map[string]release.Package,
	versions map[string][]release.Bundle,
) (*Manager, *idepkgtest.Notifications, *idepkgtest.ReleaseManager, string, document.Service) {
	temp, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(temp)
	})
	configPath := filepath.Join(temp, "config.yaml")
	n := idepkgtest.NewNotifications(t)
	m := idepkgtest.NewReleaseManager(packages, versions)
	fileScheme := newLocalScheme(temp)
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			h.Resize(70, 20)
			h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
			return &mockWindow{}, nil
		},
	}
	storage := document.NewInMemoryService()
	manager := NewManager(n, m, storage,
		fileScheme, temp, configPath, wm, syncTick, term.NopInterrupter())
	return manager, n, m, temp, storage
}

// --- Reconcile tests ---

func TestReconcile(t *testing.T) {
	t.Parallel()

	t.Run("crash_after_storage_create_before_download", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryNotExists(t, storage, "go", "1")
		_, serr := os.Stat(makePackageVersionDirname(datadir, "go", "1"))
		assert.True(t, os.IsNotExist(serr))
	})

	t.Run("crash_during_extraction", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")
		// Create partial pkg dir
		pkgDir := makePackageVersionDirname(datadir, "go", "1")
		require.NoError(t, os.MkdirAll(pkgDir, 0777))
		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "partial.txt"), []byte("x"), 0644))

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryNotExists(t, storage, "go", "1")
		_, serr := os.Stat(pkgDir)
		assert.True(t, os.IsNotExist(serr))
	})

	t.Run("crash_after_extraction_before_storage_update", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")
		pkgDir := makePackageVersionDirname(datadir, "go", "1")
		require.NoError(t, os.MkdirAll(pkgDir, 0777))

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryNotExists(t, storage, "go", "1")
		_, serr := os.Stat(pkgDir)
		assert.True(t, os.IsNotExist(serr))
	})

	t.Run("crash_during_link_lib_copy_bin", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")
		pkgDir := makePackageVersionDirname(datadir, "go", "1")
		require.NoError(t, os.MkdirAll(pkgDir, 0777))
		libLink := makePackageLibDirname(datadir, "go")
		require.NoError(t, os.Symlink(pkgDir, libLink))

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryNotExists(t, storage, "go", "1")
		_, serr := os.Stat(pkgDir)
		assert.True(t, os.IsNotExist(serr))
		_, serr = os.Lstat(libLink)
		assert.True(t, os.IsNotExist(serr))
	})

	t.Run("staging_dir_leftover", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")
		stagingDir := makeStagingDirname(datadir, "go", "1")
		require.NoError(t, os.MkdirAll(stagingDir, 0777))

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		_, serr := os.Stat(stagingDir)
		assert.True(t, os.IsNotExist(serr))
		assertStorageEntryNotExists(t, storage, "go", "1")
	})

	t.Run("stale_tmp_symlink", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, _ := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		tmpLink := filepath.Join(datadir, "lib", "go.tmp")
		require.NoError(t, os.Symlink("/nonexistent", tmpLink))

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		_, serr := os.Lstat(tmpLink)
		assert.True(t, os.IsNotExist(serr))
	})

	t.Run("multiple_packages_one_stale", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _, storage := newTestManagerWithStorage(t, pkgs, versions)

		// Install "go:1" fully
		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Create stale "testpkg:1"
		createStaleEntry(t, storage, "testpkg", "1")

		err = m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryComplete(t, storage, "go", "1")
		assertStorageEntryNotExists(t, storage, "testpkg", "1")
	})

	t.Run("no_incomplete_installs", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _, storage := newTestManagerWithStorage(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		err = m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryComplete(t, storage, "go", "1")
	})

	t.Run("empty_storage", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, _, _ := newTestManagerWithStorage(t, pkgs, versions)

		err := m.Reconcile(context.Background())
		require.NoError(t, err)
	})

	t.Run("all_entries_stale", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")
		createStaleEntry(t, storage, "testpkg", "1")

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryNotExists(t, storage, "go", "1")
		assertStorageEntryNotExists(t, storage, "testpkg", "1")
	})

	t.Run("complete_entry_missing_dir", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createCompleteEntry(t, storage, "go", "1")
		// Don't create the pkg dir — simulates dir deletion

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		assertStorageEntryNotExists(t, storage, "go", "1")
	})

	t.Run("then_install_succeeds", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		n.SetWg(1)
		err = m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		assertStorageEntryComplete(t, storage, "go", "1")
		assertDataDirExists(t, datadir, "go")
	})

	t.Run("then_delete_returns_not_installed", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		err = m.DeletePackageVersion(context.Background(), "go", "1", false)
		require.Equal(t, ErrNotInstalled, err)
	})

	t.Run("then_list_excludes_recovered", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles()
		m, _, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "testpkg", "1")

		err := m.Reconcile(context.Background())
		require.NoError(t, err)

		it, err := m.ListInstalledPackages(context.Background())
		require.NoError(t, err)
		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Empty(t, installed)
	})
}

// --- Install stale entry handling tests ---

func TestInstallStaleEntry(t *testing.T) {
	t.Parallel()

	t.Run("detects_and_cleans_stale_entry", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)

		createStaleEntry(t, storage, "go", "1")

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		assertStorageEntryComplete(t, storage, "go", "1")
		assertDataDirExists(t, datadir, "go")
	})

	t.Run("stale_entry_with_partial_files", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")
		pkgDir := makePackageVersionDirname(datadir, "go", "1")
		require.NoError(t, os.MkdirAll(pkgDir, 0777))
		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "partial.txt"), []byte("x"), 0644))

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		assertStorageEntryComplete(t, storage, "go", "1")
		assertDataDirExists(t, datadir, "go")
	})

	t.Run("completed_entry_rejects_reinstall", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _, _ := newTestManagerWithStorage(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		err = m.InstallPackageVersion(context.Background(), "go", "1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already been installed")
	})

	t.Run("stale_entry_with_staging_dir", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)
		require.NoError(t, makePkgDirs(datadir))

		createStaleEntry(t, storage, "go", "1")
		stagingDir := makeStagingDirname(datadir, "go", "1")
		require.NoError(t, os.MkdirAll(stagingDir, 0777))

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		assertStorageEntryComplete(t, storage, "go", "1")
		_, serr := os.Stat(stagingDir)
		assert.True(t, os.IsNotExist(serr))
	})
}

// --- Listing methods filter incomplete tests ---

func TestListInstalledPackagesExcludesStale(t *testing.T) {
	t.Parallel()

	t.Run("excludes_stale", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _, storage := newTestManagerWithStorage(t, pkgs, versions)

		// Install "go:1" fully
		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		// Create stale "testpkg:1"
		createStaleEntry(t, storage, "testpkg", "1")

		it, err := m.ListInstalledPackages(context.Background())
		require.NoError(t, err)
		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		assert.Equal(t, []string{"go"}, installed)
	})
}

func TestListInstalledPackageVersionsExcludesStale(t *testing.T) {
	t.Parallel()

	t.Run("excludes_stale", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles(
			[]release.Bundle{
				{Package: "go", Version: "1"},
				{Package: "go", Version: "2"},
			},
		)
		m, n, _, _, storage := newTestManagerWithStorage(t, pkgs, versions)

		// Install "go:1" fully
		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		// Create stale "go:2"
		createStaleEntry(t, storage, "go", "2")

		it, err := m.ListInstalledPackageVersions(context.Background(), "go")
		require.NoError(t, err)
		installed, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		assert.Equal(t, []release.Version{"1"}, installed)
	})
}

func TestUsePackageVersionRejectsIncomplete(t *testing.T) {
	t.Parallel()

	t.Run("rejects_incomplete", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles(
			[]release.Bundle{
				{Package: "go", Version: "1"},
				{Package: "go", Version: "2"},
			},
		)
		m, n, _, _, storage := newTestManagerWithStorage(t, pkgs, versions)

		// Install "go:1" fully
		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		// Create stale "go:2"
		createStaleEntry(t, storage, "go", "2")

		err = m.UsePackageVersion(context.Background(), "go", "2")
		require.Equal(t, ErrNotInstalled, err)
	})
}

func TestProcessInstalledSettingsSkipsIncomplete(t *testing.T) {
	t.Parallel()

	t.Run("skips_incomplete", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "configpkg", Version: "1"}})
		m, n, _, datadir, storage := newTestManagerWithStorage(t, pkgs, versions)

		configPath := filepath.Join(datadir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0644))

		// Install "configpkg:1" fully
		n.SetWg(2)
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Create stale "configpkg:2" — processInstalledSettings should skip it
		createStaleEntry(t, storage, "configpkg", "2")

		n.ClearWg()
		err = m.ProcessInstalledSettings(context.Background())
		require.NoError(t, err)
	})
}

// --- Atomic operation tests ---

func TestInstallAtomicOperations(t *testing.T) {
	t.Parallel()

	t.Run("no_staging_dir_after_success", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, datadir, _ := newTestManagerWithStorage(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		stagingDir := makeStagingDirname(datadir, "go", "1")
		_, serr := os.Stat(stagingDir)
		assert.True(t, os.IsNotExist(serr), "staging dir should not exist after success")
	})

	t.Run("storage_entry_is_complete_after_success", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
		m, n, _, _, storage := newTestManagerWithStorage(t, pkgs, versions)

		n.SetWg(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wait()

		assertStorageEntryComplete(t, storage, "go", "1")
	})
}

func TestProcessConfigSkipsPromptWhenAlreadyMerged(t *testing.T) {
	t.Parallel()
	t.Run("no prompt shown when config is already merged", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages()
		versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "configpkg", Version: "1"}})
		m, n, _, datadir := newTestManager(t, pkgs, versions)

		// Create an existing user config so processConfig merges into it
		configPath := filepath.Join(datadir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0644))

		// First install: prompt is shown and accepted (default mock auto-accepts)
		n.SetWg(2) // apply success + download success
		err := m.InstallPackageVersion(context.Background(), "configpkg", "1")
		require.NoError(t, err)
		n.Wait()
		n.RequireNoErrorNotification()

		// Verify config was written
		_ = readUserConfig(t, datadir)

		// Override wm to panic if prompt is shown — it should NOT be called
		// since the config is already merged.
		m.wm = &mockWindowManager{
			floatingFn: func(_ browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
				t.Fatal("prompt should not be shown when config is already merged")
				return nil, nil
			},
		}

		// Re-process settings: should detect config is already merged and skip
		err = m.ProcessInstalledSettings(context.Background())
		require.NoError(t, err)
	})
}
