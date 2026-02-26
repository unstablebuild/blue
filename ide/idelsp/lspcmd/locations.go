// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package lspcmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

type locationEntry struct {
	uri     string
	rng     semanticapi.Range
	display string
}

func navigateTo(
	e locationEntry, opener browserapi.ResourceOpener,
	wm browserapi.WindowManager,
	editor textapi.Editor, notify browserapi.Notifications,
	scheduleNextTick func(func()) bool,
) {
	scheduleNextTick(func() {
		if err := doNavigate(e, opener, wm, editor); err != nil {
			_, _ = notify.Notify(browserapi.LevelError, "navigate: %s", err)
		}
	})
}

func doNavigate(
	e locationEntry, opener browserapi.ResourceOpener,
	wm browserapi.WindowManager, editor textapi.Editor,
) error {
	uri, err := lspToURI(e.uri)
	if err != nil {
		return err
	}
	h, err := opener.Open(uri)
	if err != nil {
		return err
	}
	w, err := wm.Focus()
	if err != nil {
		return err
	}
	if err := wm.SetWindowContent(w, h); err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		return err
	}
	eh, err := editor.Editor(uri)
	if err != nil {
		return err
	}
	return editor.SetCursor(eh, posToCoord(e.rng.Start))
}

func locationsFromResult(r semanticapi.LocationResult) []locationEntry {
	var entries []locationEntry
	if r.Location != nil {
		entries = append(entries, locationFromLoc(*r.Location))
	}
	for _, loc := range r.Locations {
		entries = append(entries, locationFromLoc(loc))
	}
	for _, ll := range r.LocationLinks {
		entries = append(entries, locationEntry{
			uri: ll.TargetURI,
			rng: ll.TargetSelectionRange,
			display: fmt.Sprintf("%s:%d",
				trimFilePrefix(ll.TargetURI), ll.TargetSelectionRange.Start.Line+1),
		})
	}
	return entries
}

func locationFromLoc(loc semanticapi.Location) locationEntry {
	return locationEntry{
		uri:     loc.URI,
		rng:     loc.Range,
		display: fmt.Sprintf("%s:%d", trimFilePrefix(loc.URI), loc.Range.Start.Line+1),
	}
}

func trimFilePrefix(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

func enrichEntries(
	entries []locationEntry, rootURI workspaceapi.URI,
) []locationEntry {
	for i, e := range entries {
		uri, err := lspToURI(e.uri)
		var rel string
		if err == nil {
			rel = workspaceapi.RelPath(rootURI, uri)
		} else {
			rel = trimFilePrefix(e.uri)
		}
		entries[i].display = fmt.Sprintf("%s:%d", rel, e.rng.Start.Line+1)
	}
	return entries
}
