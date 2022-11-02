package firstmover

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/test"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	A string
}

func testConfig() Config {
	return Config{
		TransientFailureRecoverTimeout: 250 * time.Millisecond,
		MethodRetryCadence:             20 * time.Millisecond,
		ConnectRetryCadence:            50 * time.Millisecond,
		TimeToCoup:                     350 * time.Millisecond,
		DialTimeout:                    50 * time.Millisecond,
	}
}

func TestServiceIntegration(t *testing.T) {
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.TraceLevel)
	t.Run("single instance assumes leader", func(t *testing.T) {
		test.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := ioutil.TempFile("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			require.NoError(t, os.Remove(f.Name()))
			svc := document.NewInMemoryService()
			return New(svc, f.Name(), testConfig())
		})
	})

	t.Run("two instances, seconds assumes follower", func(t *testing.T) {
		test.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := ioutil.TempFile("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			require.NoError(t, os.Remove(f.Name()))
			svc := document.NewInMemoryService()
			leader := New(svc, f.Name(), testConfig())
			// ensure leader is available
			var temp testStruct
			err = leader.Get(context.Background(), f.Name(), &temp)
			require.Equal(t, document.ErrNotFound, err)
			follower := New(svc, f.Name(), testConfig())
			return follower
		})
	})

	t.Run("single instance eventually assumes leader if leader is non-responsive (lock leaked)", func(t *testing.T) {
		test.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := ioutil.TempFile("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			// do not remove file
			svc := document.NewInMemoryService()
			return New(svc, f.Name(), testConfig())
		})
	})

	t.Run("two instances, seconds assumes leader after leader dies", func(t *testing.T) {
		test.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := ioutil.TempFile("", "")
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

			f, err := ioutil.TempFile("", "")
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
			return instances[len(instances)-1]
		})
	})
}
