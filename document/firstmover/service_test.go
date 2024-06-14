package firstmover

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/test"
	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/encoding/bson"
	"github.com/unstablebuild/blue/encoding/json"
	"github.com/unstablebuild/blue/encoding/toml"
)

func TestServiceIntegration(t *testing.T) {
	for name, _marshaler := range map[string]encoding.Marshaler{
		"bson": bson.Marshaler(),
		"json": json.Marshaler(),
		"toml": toml.Marshaler(),
	} {
		marshaler := _marshaler
		t.Run(name, func(t *testing.T) {
			t.Run("single instance assumes leader", func(t *testing.T) {
				test.TestDocumentService(t, func(t *testing.T) document.Service {
					f, err := os.CreateTemp("", "")
					require.NoError(t, err)
					require.NoError(t, f.Close())
					require.NoError(t, os.Remove(f.Name()))
					cfg := testConfig()
					cfg.Marshaler = marshaler
					svc := document.NewInMemoryServiceWithMarshaler(marshaler)
					return New(svc, f.Name(), cfg)
				})
			})

			t.Run("two instances, seconds assumes follower", func(t *testing.T) {
				test.TestDocumentService(t, func(t *testing.T) document.Service {
					f, err := os.CreateTemp("", "")
					require.NoError(t, err)
					require.NoError(t, f.Close())
					require.NoError(t, os.Remove(f.Name()))
					svc := document.NewInMemoryServiceWithMarshaler(marshaler)
					cfg := testConfig()
					cfg.Marshaler = marshaler
					leader := New(svc, f.Name(), cfg)
					// ensure leader is available
					var temp testStruct
					err = leader.Get(context.Background(), f.Name(), &temp)
					require.Equal(t, document.ErrNotFound, err)
					follower := New(svc, f.Name(), cfg)
					return follower
				})
			})
		})
	}

	t.Run("single instance preconditions (bson)", func(t *testing.T) {
		test.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			require.NoError(t, os.Remove(f.Name()))
			cfg := testConfig()
			cfg.Marshaler = bson.Marshaler()
			svc := document.NewInMemoryServiceWithMarshaler(cfg.Marshaler)
			return New(svc, f.Name(), cfg)
		})
	})

	t.Run("single instance eventually assumes leader if leader is non-responsive (lock leaked)", func(t *testing.T) {
		test.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			// do not remove file
			svc := document.NewInMemoryService()
			return New(svc, f.Name(), testConfig())
		})
	})

	t.Run("two instances, seconds assumes leader after leader dies", func(t *testing.T) {
		test.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			require.NoError(t, os.Remove(f.Name()))
			svc := document.NewInMemoryService()
			leader := New(svc, f.Name(), testConfig())
			// ensure leader is available
			err = leader.Set(context.Background(), f.Name(), &testStruct{A: "1234"})
			require.NoError(t, err)
			follower := New(document.NewInMemoryService(), f.Name(), testConfig())
			// ensure follow is available and using leader
			var temp testStruct
			err = follower.Get(context.Background(), f.Name(), &temp)
			require.NoError(t, err)
			require.Equal(t, "1234", temp.A)
			leader.Close()
			return follower
		})
	})

	t.Run("multiple instances", func(t *testing.T) {
		test.TestDocumentService(t, func(t *testing.T) document.Service {
			const n = 50
			cfg := testConfig()

			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			require.NoError(t, os.Remove(f.Name()))
			svc := document.NewInMemoryService()

			instances := make([]document.Service, 0, n)

			// chances of returned follower to become leader are ~1/50
			for i := 0; i < n-1; i++ {
				instance := New(svc, f.Name(), cfg)
				_ = instance.Get(context.Background(), f.Name(), nil)
				instances = append(instances, instance)
			}
			ret := instances[len(instances)-1]

			go func() {
				for i := 0; i < n-1; i++ { // always leave one fully operating
					time.Sleep(cfg.DialTimeout + cfg.ConnectRetryCadence)
					for idx, instance := range instances {
						if TestIsLeader(instance) {
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
			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			require.NoError(t, os.Remove(f.Name()))
			require.Error(t, tcase.methodError)
			mock := newTestService(tcase.methodError)
			cfg := testConfig()
			cfg.CloseError = tcase.closeError
			svc, doneFn := tcase.makeInstance(mock, f.Name(), cfg)
			defer doneFn()
			err = svc.Set(context.Background(), "bluegrass", &testStruct{A: "1234"})
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
		Marshaler:                      bson.Marshaler(),
		TransientFailureRecoverTimeout: 450 * time.Millisecond,
		MethodRetryCadence:             20 * time.Millisecond,
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
