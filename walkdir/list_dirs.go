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

package walkdir

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// ListDirs traverses the workspace directory and returns
// an iterator that returns all directories under root. If root is
// a partial or full file name, it will be ignored and its base
// directory, will be used. If errors are encountered while reading
// the contents of directories those errors will be aggregated and
// reported by the iterator's Err method.
func ListDirs(
	ctx context.Context, w Reader, root string,
) (iterator.Iterator[string], error) {
	var wg sync.WaitGroup
	iterCh := make(chan string)
	workerCh := make(chan string)
	closeWaitCh := make(chan struct{})
	allErrors := make([]error, defaultWorkers)

	workspaceURI, err := w.URI(".")
	if err != nil {
		return nil, fmt.Errorf("URI: %v", err)
	}
	rootURI, err := w.URI(root)
	if err != nil {
		return nil, fmt.Errorf("URI: %v", err)
	}

	// get root as relative path to workspace
	root = workspaceapi.RelPath(workspaceURI, rootURI)

	iterator := &listFilesIterator{dataCh: iterCh}
	ctx, cancel := context.WithCancel(ctx)
	iterator.ctx = ctx
	iterator.cancel = cancel
	iterator.closeWaitCh = closeWaitCh

	for i := range defaultWorkers {
		go func() {
			traverseDirWorker(ctx, w, &wg, iterCh, workerCh,
				workspaceURI.Path(), &iterator.mu, &allErrors[i], true)
		}()
	}

	for {
		finfo, err := w.Stat(root)
		if err == nil && finfo.IsDir() {
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		root = filepath.Dir(root)
	}

	wg.Add(1)
	workerCh <- root

	go func() {
		defer close(closeWaitCh)
		defer close(iterCh)
		defer close(workerCh)

		wg.Wait()
		iterator.mu.Lock()
		defer iterator.mu.Unlock()
		for _, err := range allErrors {
			if err != nil {
				iterator.err = multierr.Append(iterator.err, err)
			}
		}
	}()

	return iterator, nil
}
