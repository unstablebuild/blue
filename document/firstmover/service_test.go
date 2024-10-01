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

package firstmover

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/document/doctest"
)

func TestServiceIntegration(t *testing.T) {
	for name, _marshaler := range map[string]docmarshal.Marshaler{
		"bson": docbson.Marshaler(),
		"json": docjson.Marshaler(),
		"toml": doctoml.Marshaler(),
	} {
		marshaler := _marshaler
		t.Run(name, func(t *testing.T) {
			t.Run("single instance assumes leader", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					lockFile := makeTempLockFile(t)
					cfg := testConfig()
					cfg.Marshaler = marshaler
					svc := document.NewInMemoryServiceWithMarshaler(marshaler)
					return New(svc, lockFile, cfg)
				})
			})

			t.Run("two instances, seconds assumes follower", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					lockFile := makeTempLockFile(t)
					svc := document.NewInMemoryServiceWithMarshaler(marshaler)
					cfg := testConfig()
					cfg.Marshaler = marshaler
					leader := New(svc, lockFile, cfg)
					// ensure leader is available
					var temp testStruct
					err := leader.Get(context.Background(), lockFile, &temp)
					require.Equal(t, document.ErrNotFound, err)
					follower := New(svc, lockFile, cfg)
					return follower
				})
			})
		})
	}

	t.Run("single instance with lock path on non-existent folder "+
		"attempts to create directory structure", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			lockFileDir := filepath.Join(filepath.Dir(lockFile), "newDir", "otherDir", "moreDirs")
			lockFile = filepath.Join(lockFileDir, ".lock")
			cfg := testConfig()
			cfg.Marshaler = docbson.Marshaler()
			svc := document.NewInMemoryServiceWithMarshaler(cfg.Marshaler)
			return New(svc, lockFile, cfg)
		})
	})

	t.Run("single instance preconditions (bson)", func(t *testing.T) {
		doctest.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			cfg := testConfig()
			cfg.Marshaler = docbson.Marshaler()
			svc := document.NewInMemoryServiceWithMarshaler(cfg.Marshaler)
			return New(svc, lockFile, cfg)
		})
	})

	t.Run("single instance eventually assumes leader if leader is non-responsive (lock leaked)", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			// do not remove file
			svc := document.NewInMemoryService()
			return New(svc, f.Name(), testConfig())
		})
	})

	t.Run("two instances, seconds assumes leader after leader dies", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			svc := document.NewInMemoryService()
			leader := New(svc, lockFile, testConfig())
			// ensure leader is available
			err := leader.Set(context.Background(), lockFile, &testStruct{A: "1234"})
			require.NoError(t, err)
			follower := New(document.NewInMemoryService(), lockFile, testConfig())
			// ensure follow is available and using leader
			var temp testStruct
			err = follower.Get(context.Background(), lockFile, &temp)
			require.NoError(t, err)
			require.Equal(t, "1234", temp.A)
			leader.Close()
			return follower
		})
	})

	t.Run("remove leader lock, seconds assumes leader after leader is unresponsive", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			svc := document.NewInMemoryService()
			leader := New(svc, lockFile, testConfig())
			// ensure leader is available
			err := leader.Set(context.Background(), "dragonballz", &testStruct{A: "1234"})
			require.NoError(t, err)
			follower := New(document.NewInMemoryService(), lockFile, testConfig())
			// ensure follow is available and using leader
			var temp testStruct
			err = follower.Get(context.Background(), "dragonballz", &temp)
			require.NoError(t, err)
			require.Equal(t, "1234", temp.A)
			err = follower.Delete(context.Background(), "dragonballz")
			require.NoError(t, err)

			// remove lock
			os.Remove(lockFile)
			return follower
		})
	})

	t.Run("multiple instances", func(t *testing.T) {
		doctest.TestDocumentServiceNoList(t, func(t *testing.T) document.Service {
			const n = 100 // have a n ~100 reproduces certain shutdown/recovery issues
			cfg := testConfig()

			lockFile := makeTempLockFile(t)
			svc := document.NewInMemoryService()

			instances := make([]*Service, 0, n)

			// chances of returned follower to become leader are ~1/50
			for i := 0; i < n-1; i++ {
				instance := New(svc, lockFile, cfg)
				_ = instance.Get(context.Background(), lockFile, nil)
				instances = append(instances, instance)
			}
			ret := instances[len(instances)-1]

			go func() {
				for i := 0; i < n-1; i++ { // always leave one fully operating
					time.Sleep(cfg.DialTimeout + cfg.ConnectRetryCadence)
					for idx, instance := range instances {
						if instance.IsLeader() {
							instance.Close()
							if idx == len(instances)-1 {
								instances = instances[:idx]
							} else {
								instances = append(instances[:idx], instances[idx+1:]...)
							}
							break
						}
					}
				}
			}()
			return ret
		})
	})

	t.Run("Close on massive network of peers", func(t *testing.T) {
		const n = 100
		cfg := testConfig()

		lockFile := makeTempLockFile(t)
		svc := document.NewInMemoryService()

		instances := make([]*Service, 0, n)
		for i := 0; i < n; i++ {
			instance := New(svc, lockFile, cfg)
			_ = instance.Get(context.Background(), lockFile, nil)
			instances = append(instances, instance)
		}
		for i := 0; i < n; i++ {
			instance := instances[i]
			require.NoError(t, instance.Close())
		}
	})
}

func TestCustomRetryableErrors(t *testing.T) {
	myError := errors.New("dia de los muertos")
	tsuite := []struct {
		desc         string
		methodError  error
		closeError   error
		wantSuccess  bool
		makeInstance func(document.Service, string, Config) (document.Service, func())
	}{
		{"leader does not retry error passed in Config", myError, myError, false, makeLeader},
		{"follower retries close error passed in Config until success", myError, myError, true, makeFollower},
		{"follower does not retry any error other than the error passed in Config", errors.New("wasup"), myError, false, makeFollower},
		{"follower does not retry any error if error in config is nil", errors.New("wasup"), nil, false, makeFollower},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			lockFile := makeTempLockFile(t)
			require.Error(t, tcase.methodError)
			mock := newTestService(tcase.methodError)
			cfg := testConfig()
			cfg.CloseError = tcase.closeError
			svc, doneFn := tcase.makeInstance(mock, lockFile, cfg)
			defer doneFn()
			err := svc.Set(context.Background(), "bluegrass", &testStruct{A: "1234"})
			if !tcase.wantSuccess {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func makeFollower(svc document.Service, lockFile string, cfg Config) (document.Service, func()) {
	var temp testStruct
	leader := New(svc, lockFile, cfg)
	// ensure leader is ready
	_ = leader.Get(context.Background(), "bla", &temp)
	follower := New(svc, lockFile, cfg)
	// ensure follower is ready
	_ = follower.Get(context.Background(), "bla", &temp)
	return follower, func() {
		leader.Close()
		follower.Close()
	}
}

func makeLeader(svc document.Service, lockFile string, cfg Config) (document.Service, func()) {
	leader := New(svc, lockFile, cfg)
	return leader, func() {
		_ = leader.Close()
	}
}

type testStruct struct {
	A string
}

func testConfig() Config {
	return Config{
		Marshaler:                      docbson.Marshaler(),
		TransientFailureRecoverTimeout: 450 * time.Millisecond,
		MethodRetryCadence:             20 * time.Millisecond,
		ReceiveRetryCadence:            500 * time.Millisecond,
		ConnectRetryCadence:            50 * time.Millisecond,
		TimeToCoup:                     500 * time.Millisecond,
		DialTimeout:                    50 * time.Millisecond,
	}
}

type testService struct {
	err   error
	tries int
	svc   document.Service
}

func newTestService(err error) document.Service {
	return &testService{err: err, svc: document.NewInMemoryService()}
}

func (t *testService) Set(ctx context.Context, ID string, doc interface{}) error {
	if t.err == nil {
		panic("incorrect test case")
	}
	t.tries++
	if t.tries < 2 {
		return t.err
	}
	return t.svc.Set(ctx, ID, doc)
}

func (t *testService) Get(ctx context.Context, ID string, doc interface{}) error {
	return t.svc.Get(ctx, ID, doc)
}

func (t *testService) Create(ctx context.Context, ID string, doc interface{}) error {
	panic("unimplemented")
}
func (t *testService) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	panic("unimplemented")
}

func (t *testService) Delete(ctx context.Context, ID string) error {
	panic("unimplemented")
}

func (t *testService) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	panic("unimplemented")
}

func (t *testService) Close() error {
	return nil
}

func makeTempLockFile(t *testing.T) string {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Remove(f.Name()))
	t.Cleanup(func() {
		os.Remove(f.Name())
	})
	return f.Name()
}
