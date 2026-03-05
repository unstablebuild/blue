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
	"slices"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// Update represents an available package update.
type Update struct {
	Package string
	Current release.Version
	Latest  release.Version
}

// UpdateChecker detects available updates for installed packages
// and notifies/prompts the user.
type UpdateChecker struct {
	m      *Manager
	now    func() time.Time
	cancel context.CancelFunc
}

// NewUpdateChecker returns a new UpdateChecker.
func NewUpdateChecker(m *Manager) *UpdateChecker {
	return &UpdateChecker{m: m, now: time.Now}
}

// Close cancels any in-flight update check started by Start.
func (uc *UpdateChecker) Close() error {
	if uc.cancel != nil {
		uc.cancel()
	}
	return nil
}

// CheckForUpdates compares installed package versions against the registry
// and returns available updates.
func (uc *UpdateChecker) CheckForUpdates(ctx context.Context) ([]Update, error) {
	it, err := uc.m.ListInstalledPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installed packages: %w", err)
	}

	seen := make(map[string]struct{})
	var updates []Update

	for {
		pkgID, ok := it.Next(ctx)
		if !ok {
			break
		}
		if pkgID == "" {
			continue
		}
		if _, dup := seen[pkgID]; dup {
			continue
		}
		seen[pkgID] = struct{}{}

		u, err := uc.checkPackage(ctx, pkgID)
		if err != nil {
			uc.m.log(log.WarnLevel, "update check for %s: %v", pkgID, err)
			continue
		}
		if u != nil {
			updates = append(updates, *u)
		}
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("installed packages iterator: %w", err)
	}

	return updates, nil
}

func (uc *UpdateChecker) checkPackage(ctx context.Context, pkgID string) (*Update, error) {
	vit, err := uc.m.ListInstalledPackageVersions(ctx, pkgID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	var current release.Version
	for {
		v, ok := vit.Next(ctx)
		if !ok {
			break
		}
		_, _, inUse, err := uc.m.isPackageVersionInUse(pkgID, v)
		if err != nil {
			continue
		}
		if inUse {
			current = v
			break
		}
	}
	if err := vit.Err(); err != nil {
		return nil, fmt.Errorf("versions iterator: %w", err)
	}
	if current == "" {
		return nil, nil
	}

	pkg, err := uc.m.DescribePackage(ctx, pkgID)
	if err != nil {
		return nil, fmt.Errorf("describe package: %w", err)
	}
	if pkg.Latest == "" || release.Version(pkg.Latest) == current {
		return nil, nil
	}

	return &Update{
		Package: pkgID,
		Current: current,
		Latest:  release.Version(pkg.Latest),
	}, nil
}

// Start runs the update check flow in a background goroutine.
// Use Close to cancel the check if it's still running.
func (uc *UpdateChecker) Start(ctx context.Context) {
	ctx, uc.cancel = context.WithCancel(ctx)
	go uc.m.capturePanicReport(func() {
		uc.run(ctx)
	})
}

const (
	updateCheckLastKey    = "update-check:last"
	updateAvailablePrefix = "update-available:"
	updateAvailableIndex  = "update-available:__index__"
	throttleDuration      = 24 * time.Hour
	promptAfterDuration   = 7 * 24 * time.Hour
)

type updateCheckValue struct {
	Timestamp time.Time
}

type updateAvailableValue struct {
	Package   string
	Current   release.Version
	Latest    release.Version
	FirstSeen time.Time
	Skipped   bool
}

type updateAvailableIndexValue struct {
	Packages []string
}

func (uc *UpdateChecker) run(ctx context.Context) {
	if uc.isThrottled(ctx) {
		return
	}

	updates, err := uc.CheckForUpdates(ctx)
	if err != nil {
		uc.m.log(log.WarnLevel, "check for updates: %v", err)
		return
	}

	uc.persistThrottle(ctx)

	if len(updates) == 0 {
		uc.cleanupStaleRecords(ctx, nil)
		return
	}

	currentPkgIDs := make(map[string]struct{}, len(updates))
	for _, u := range updates {
		currentPkgIDs[u.Package] = struct{}{}
		uc.upsertUpdateRecord(ctx, u)
	}

	uc.cleanupStaleRecords(ctx, currentPkgIDs)

	summary := formatUpdateSummary(updates)
	if _, err := uc.m.n.NotifyOnce(browserapi.LevelInfo, summary); err != nil {
		uc.m.log(log.WarnLevel, "notify updates: %v", err)
	}

	promptUpdates := uc.filterPromptUpdates(ctx, updates)
	if len(promptUpdates) > 0 {
		uc.showUpdatePrompt(ctx, promptUpdates)
	}
}

func (uc *UpdateChecker) isThrottled(ctx context.Context) bool {
	var val updateCheckValue
	err := uc.m.storage.Get(ctx, updateCheckLastKey, &val)
	if err != nil {
		return false
	}
	return uc.now().Sub(val.Timestamp) < throttleDuration
}

func (uc *UpdateChecker) persistThrottle(ctx context.Context) {
	val := updateCheckValue{Timestamp: uc.now()}
	if err := uc.m.storage.Set(ctx, updateCheckLastKey, val); err != nil {
		uc.m.log(log.WarnLevel, "persist throttle: %v", err)
	}
}

func (uc *UpdateChecker) upsertUpdateRecord(ctx context.Context, u Update) {
	key := updateAvailablePrefix + u.Package
	var existing updateAvailableValue
	err := uc.m.storage.Get(ctx, key, &existing)

	now := uc.now()
	if err != nil {
		// new record
		val := updateAvailableValue{
			Package:   u.Package,
			Current:   u.Current,
			Latest:    u.Latest,
			FirstSeen: now,
		}
		if err := uc.m.storage.Set(ctx, key, val); err != nil {
			uc.m.log(log.WarnLevel, "upsert update record %s: %v", u.Package, err)
		}
		uc.addToIndex(ctx, u.Package)
		return
	}

	if existing.Latest == u.Latest {
		// same version — update current only
		existing.Current = u.Current
		if err := uc.m.storage.Set(ctx, key, existing); err != nil {
			uc.m.log(log.WarnLevel, "upsert update record %s: %v", u.Package, err)
		}
		return
	}

	// newer version available — reset
	val := updateAvailableValue{
		Package:   u.Package,
		Current:   u.Current,
		Latest:    u.Latest,
		FirstSeen: now,
	}
	if err := uc.m.storage.Set(ctx, key, val); err != nil {
		uc.m.log(log.WarnLevel, "upsert update record %s: %v", u.Package, err)
	}
	uc.addToIndex(ctx, u.Package)
}

func (uc *UpdateChecker) addToIndex(ctx context.Context, pkgID string) {
	var idx updateAvailableIndexValue
	_ = uc.m.storage.Get(ctx, updateAvailableIndex, &idx)

	if slices.Contains(idx.Packages, pkgID) {
		return
	}
	idx.Packages = append(idx.Packages, pkgID)
	if err := uc.m.storage.Set(ctx, updateAvailableIndex, idx); err != nil {
		uc.m.log(log.WarnLevel, "update index: %v", err)
	}
}

func (uc *UpdateChecker) cleanupStaleRecords(ctx context.Context, currentPkgIDs map[string]struct{}) {
	var idx updateAvailableIndexValue
	if err := uc.m.storage.Get(ctx, updateAvailableIndex, &idx); err != nil {
		return
	}

	var remaining []string
	for _, pkgID := range idx.Packages {
		if currentPkgIDs != nil {
			if _, ok := currentPkgIDs[pkgID]; ok {
				remaining = append(remaining, pkgID)
				continue
			}
		}
		key := updateAvailablePrefix + pkgID
		_ = uc.m.storage.Delete(ctx, key)
	}

	if len(remaining) == 0 {
		_ = uc.m.storage.Delete(ctx, updateAvailableIndex)
		return
	}

	idx.Packages = remaining
	if err := uc.m.storage.Set(ctx, updateAvailableIndex, idx); err != nil {
		uc.m.log(log.WarnLevel, "cleanup index: %v", err)
	}
}

func (uc *UpdateChecker) filterPromptUpdates(ctx context.Context, updates []Update) []Update {
	var result []Update
	now := uc.now()
	for _, u := range updates {
		key := updateAvailablePrefix + u.Package
		var val updateAvailableValue
		if err := uc.m.storage.Get(ctx, key, &val); err != nil {
			continue
		}
		if val.Skipped {
			continue
		}
		if now.Sub(val.FirstSeen) >= promptAfterDuration {
			result = append(result, u)
		}
	}
	return result
}

func formatUpdateSummary(updates []Update) string {
	var b strings.Builder
	b.WriteString("Updates available:")
	for _, u := range updates {
		fmt.Fprintf(&b, "\n  %s: %s → %s", u.Package, u.Current, u.Latest)
	}
	return b.String()
}

func (uc *UpdateChecker) showUpdatePrompt(ctx context.Context, updates []Update) {
	message := formatUpdateSummary(updates) + "\n\nInstall updates?"

	prompt := handler.NewPrompt(handler.PromptConfig{
		HighlightAttr: term.Attributes{
			Attrs: tcell.AttrBold,
			Fg:    tcell.ColorBlue,
		},
		OptionBindings: []term.KeyComb{{Ch: 'u'}, {Ch: 'r'}, {Ch: 's'}},
		PromptConfig: component.PromptConfig{
			Message: message,
			Options: []string{"Upgrade All", "Remind Later", "Skip"},
			Frame:   component.FrameCharSetDefault(),
		},
		PromptHandler: handler.FuncPromptHandler(func(idx int, _ string) {
			switch idx {
			case 0: // Update All
				for _, u := range updates {
					if err := uc.m.InstallPackageVersion(ctx, u.Package, u.Latest); err != nil {
						uc.m.log(log.WarnLevel, "install update %s %s: %v", u.Package, u.Latest, err)
					}
				}
			case 2: // Skip These Versions
				for _, u := range updates {
					if err := uc.SkipVersion(ctx, u.Package, u.Latest); err != nil {
						uc.m.log(log.WarnLevel, "skip version %s %s: %v", u.Package, u.Latest, err)
					}
				}
			}
			// case 1 (Remind Later): no-op
		}, func() error {
			return nil
		}),
	})

	uc.m.scheduleNextTick(func() {
		_, err := uc.m.wm.Floating(prompt, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		})
		if err != nil {
			uc.m.log(log.WarnLevel, "show update prompt: %v", err)
		}
	})
}

// SkipVersion marks a specific version of a package as skipped,
// so the user won't be prompted to install it.
func (uc *UpdateChecker) SkipVersion(ctx context.Context, pkgID string, version release.Version) error {
	key := updateAvailablePrefix + pkgID
	var val updateAvailableValue
	err := uc.m.storage.Get(ctx, key, &val)
	if err != nil {
		if errors.Is(err, document.ErrNotFound) {
			val = updateAvailableValue{
				Package:   pkgID,
				Latest:    version,
				FirstSeen: uc.now(),
				Skipped:   true,
			}
			return uc.m.storage.Set(ctx, key, val)
		}
		return err
	}

	if val.Latest != version {
		return nil
	}

	val.Skipped = true
	return uc.m.storage.Set(ctx, key, val)
}
