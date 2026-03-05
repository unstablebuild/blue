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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/ide/idepkg/idepkgtest"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestCheckForUpdates(t *testing.T) {
	t.Parallel()

	t.Run("returns updates when newer version available", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)
		updates, err := uc.CheckForUpdates(context.Background())
		require.NoError(t, err)
		require.Len(t, updates, 1)
		assert.Equal(t, "go", updates[0].Package)
		assert.Equal(t, release.Version("1"), updates[0].Current)
		assert.Equal(t, release.Version("2"), updates[0].Latest)
	})

	t.Run("returns empty when up to date", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "1"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)
		updates, err := uc.CheckForUpdates(context.Background())
		require.NoError(t, err)
		assert.Empty(t, updates)
	})

	t.Run("skips erroring packages", func(t *testing.T) {
		t.Parallel()
		// "go" package exists in registry but "testpkg" does not have a Package entry
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles(
			[]release.Bundle{{Package: "go", Version: "1"}},
			[]release.Bundle{{Package: "testpkg", Version: "1"}},
		)
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(2)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		err = m.InstallPackageVersion(context.Background(), "testpkg", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)
		updates, err := uc.CheckForUpdates(context.Background())
		require.NoError(t, err)
		require.Len(t, updates, 1)
		assert.Equal(t, "go", updates[0].Package)
	})

	t.Run("deduplicates packages", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "3"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err = m.InstallPackageVersion(context.Background(), "go", "2")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)
		updates, err := uc.CheckForUpdates(context.Background())
		require.NoError(t, err)
		// should only return one update for "go", not two
		require.Len(t, updates, 1)
		assert.Equal(t, "go", updates[0].Package)
	})
}

func TestStart(t *testing.T) {
	t.Parallel()

	t.Run("respects 24h throttle", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		now := time.Now()
		uc := NewUpdateChecker(m)
		uc.now = func() time.Time { return now }

		// First run should proceed and notify
		n.Reset()
		uc.run(context.Background())

		active := n.Active()
		assert.NotEmpty(t, active, "expected notification after first run")

		// Second run with same time should be throttled — no new notifications
		n.Reset()
		uc.run(context.Background())

		active = n.Active()
		assert.Empty(t, active, "expected no notification due to throttle")
	})

	t.Run("sends notification when updates available", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)

		n.Reset()
		uc.run(context.Background())

		active := n.Active()
		require.NotEmpty(t, active)
		var found bool
		for _, noti := range active {
			if noti.Level == browserapi.LevelInfo &&
				assert.ObjectsAreEqual("Updates available:\n  go: 1 → 2", noti.Msg) {
				found = true
			}
		}
		assert.True(t, found, "expected update notification, got: %+v", active)
	})

	t.Run("shows prompt after 7 days", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		now := time.Now()
		uc := NewUpdateChecker(m)
		uc.now = func() time.Time { return now }

		// First run — sets FirstSeen
		n.Reset()
		uc.run(context.Background())

		// Advance past 7 days and un-throttle
		now = now.Add(8 * 24 * time.Hour)

		// The prompt will auto-select "Update All" (index 0) via mockWindowManager
		// which sends Enter on the first option.
		n.Reset()
		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1) // expect install notification
		uc.run(context.Background())
		n.Wg.Wait()
	})

	t.Run("respects skipped versions", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		now := time.Now()
		uc := NewUpdateChecker(m)
		uc.now = func() time.Time { return now }

		// First run to create records
		n.Reset()
		uc.run(context.Background())

		// Skip version 2
		err = uc.SkipVersion(context.Background(), "go", "2")
		require.NoError(t, err)

		// Advance past 7 days
		now = now.Add(8 * 24 * time.Hour)

		// Run again — notification should still appear but no prompt
		// (prompt filters out skipped). Since prompt won't trigger install,
		// we shouldn't get an install notification.
		n.Reset()
		uc.run(context.Background())

		active := n.Active()
		// Should have the info notification but no install notification
		for _, noti := range active {
			assert.NotContains(t, noti.Msg, "downloading version")
		}
	})

	t.Run("cleans up stale records", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)
		storage := m.storage

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)

		// First run creates records for "go"
		n.Reset()
		uc.run(context.Background())

		// Verify record exists
		var val updateAvailableValue
		err = storage.Get(context.Background(), "update-available:go", &val)
		require.NoError(t, err)
		assert.Equal(t, "go", val.Package)

		// Now simulate "go" becoming up-to-date (Latest == installed)
		pkgs["go"] = release.Package{Name: "go", Latest: "1"}

		// Advance past throttle
		now := time.Now().Add(25 * time.Hour)
		uc.now = func() time.Time { return now }

		n.Reset()
		uc.run(context.Background())

		// Record should be cleaned up
		err = storage.Get(context.Background(), "update-available:go", &val)
		assert.ErrorIs(t, err, document.ErrNotFound, "expected stale record to be cleaned up")
	})
}

func TestSkipVersion(t *testing.T) {
	t.Parallel()

	t.Run("marks version as skipped", func(t *testing.T) {
		t.Parallel()
		m, _, _, _ := newTestManager(t,
			idepkgtest.MakePackages(), idepkgtest.MakeBundles())
		storage := m.storage

		uc := NewUpdateChecker(m)

		// Pre-create a record
		ctx := context.Background()
		key := "update-available:go"
		val := updateAvailableValue{
			Package: "go", Latest: "2", FirstSeen: time.Now(),
		}
		require.NoError(t, storage.Set(ctx, key, val))

		err := uc.SkipVersion(ctx, "go", "2")
		require.NoError(t, err)

		var got updateAvailableValue
		require.NoError(t, storage.Get(ctx, key, &got))
		assert.True(t, got.Skipped)
	})

	t.Run("no-op for different version", func(t *testing.T) {
		t.Parallel()
		m, _, _, _ := newTestManager(t,
			idepkgtest.MakePackages(), idepkgtest.MakeBundles())
		storage := m.storage

		uc := NewUpdateChecker(m)

		ctx := context.Background()
		key := "update-available:go"
		val := updateAvailableValue{
			Package: "go", Latest: "3", FirstSeen: time.Now(),
		}
		require.NoError(t, storage.Set(ctx, key, val))

		err := uc.SkipVersion(ctx, "go", "2")
		require.NoError(t, err)

		var got updateAvailableValue
		require.NoError(t, storage.Get(ctx, key, &got))
		assert.False(t, got.Skipped, "should not skip when version doesn't match")
	})

	t.Run("creates record if absent", func(t *testing.T) {
		t.Parallel()
		m, _, _, _ := newTestManager(t,
			idepkgtest.MakePackages(), idepkgtest.MakeBundles())
		storage := m.storage

		uc := NewUpdateChecker(m)

		ctx := context.Background()
		err := uc.SkipVersion(ctx, "go", "2")
		require.NoError(t, err)

		var got updateAvailableValue
		require.NoError(t, storage.Get(ctx, "update-available:go", &got))
		assert.True(t, got.Skipped)
		assert.Equal(t, release.Version("2"), got.Latest)
	})
}

func TestUpdatePromptActions(t *testing.T) {
	t.Parallel()

	t.Run("Update All triggers installs", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)

		updates := []Update{{Package: "go", Current: "1", Latest: "2"}}

		// Mock will auto-select first option (Update All) via Enter key
		n.Reset()
		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		uc.showUpdatePrompt(context.Background(), updates)
		n.Wg.Wait()

		active := n.Active()
		var found bool
		for _, noti := range active {
			if noti.Level == browserapi.LevelSuccess {
				found = true
			}
		}
		assert.True(t, found, "expected success notification from install, got: %+v", active)
	})

	t.Run("Skip persists skip", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		// Override floating to select "Skip These Versions" (index 2)
		m.wm = &mockWindowManager{
			floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
				h.Resize(70, 20)
				// Move down twice to reach "Skip These Versions"
				h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
				h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
				h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
				return &mockWindow{}, nil
			},
		}

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)
		ctx := context.Background()

		// Pre-create update record
		key := "update-available:go"
		require.NoError(t, m.storage.Set(ctx, key, updateAvailableValue{
			Package: "go", Latest: "2", FirstSeen: time.Now(),
		}))

		updates := []Update{{Package: "go", Current: "1", Latest: "2"}}

		n.Reset()
		uc.showUpdatePrompt(ctx, updates)

		var got updateAvailableValue
		require.NoError(t, m.storage.Get(ctx, key, &got))
		assert.True(t, got.Skipped, "expected version to be skipped")
	})

	t.Run("Remind Later is no-op", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: "go", Latest: "2"})
		versions := idepkgtest.MakeBundles([]release.Bundle{
			{Package: "go", Version: "1"},
		})
		m, n, _, _ := newTestManager(t, pkgs, versions)

		// Override floating to select "Remind Later" (index 1)
		m.wm = &mockWindowManager{
			floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
				h.Resize(70, 20)
				h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
				h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
				return &mockWindow{}, nil
			},
		}

		n.Wg = new(sync.WaitGroup)
		n.Wg.Add(1)
		err := m.InstallPackageVersion(context.Background(), "go", "1")
		require.NoError(t, err)
		n.Wg.Wait()

		uc := NewUpdateChecker(m)
		ctx := context.Background()

		// Pre-create update record (not skipped)
		key := "update-available:go"
		require.NoError(t, m.storage.Set(ctx, key, updateAvailableValue{
			Package: "go", Latest: "2", FirstSeen: time.Now(),
		}))

		updates := []Update{{Package: "go", Current: "1", Latest: "2"}}

		n.Reset()
		uc.showUpdatePrompt(ctx, updates)

		// Verify version is NOT skipped — still should prompt next time
		var got updateAvailableValue
		require.NoError(t, m.storage.Get(ctx, key, &got))
		assert.False(t, got.Skipped, "Remind Later should not skip")
	})
}

func TestFormatUpdateSummary(t *testing.T) {
	t.Parallel()

	updates := []Update{
		{Package: "go", Current: "1.21.0", Latest: "1.22.0"},
		{Package: "rust", Current: "1.70.0", Latest: "1.72.0"},
	}
	expected := "Updates available:\n  go: 1.21.0 → 1.22.0\n  rust: 1.70.0 → 1.72.0"
	assert.Equal(t, expected, formatUpdateSummary(updates))
}
