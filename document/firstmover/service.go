package firstmover

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/rpc"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/retry"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

const (
	transientFailureRecoverTimeout = 1 * time.Second
	retryCadence                   = 20 * time.Millisecond
	connectRetryCadence            = 50 * time.Millisecond
	timeToTakeLead                 = 1 * time.Second
	dialTimeout                    = 100 * time.Millisecond
	maxFollowFailures              = int(timeToTakeLead / (dialTimeout + connectRetryCadence))
)

var (
	retryStrategy = retry.CombinedStrategy(
		retry.SequentialStrategy(retryCadence),
		retry.LimitStrategy(uint(timeToTakeLead*2/retryCadence)),
	)
	connectRetryStrategy = retry.SequentialStrategy(connectRetryCadence)
)

type service struct {
	mu       sync.Mutex
	svc      document.Service
	lockFile string

	followFailures int
	quitCh         chan struct{}
	active         document.Service
}

// New returns a document.Service that either acquires a lock
// by creating a unix socket at lockFile and exposes svc
// to RPC clients or if it fails to acquire lock, it will connect
// to the current leader via lockFile.
//
// If the leader is closed after this Service connects to it,
// or it stops responding for more than a specificed timeout,
// all running services returned will race to re-acquire the lock and
// act as the new leader.
func New(svc document.Service, lockFile string) document.Service {
	ret := &service{svc: svc, lockFile: lockFile, quitCh: make(chan struct{})}
	ret.mu.Lock()
	go ret.leadOrFollow()
	return ret
}

func (s *service) isRetriableError(err error) bool {
	if s.svc == s.active {
		return false
	}

	c := status.Convert(err).Code()
	return c == codes.Unavailable || c == codes.DeadlineExceeded
}

// if last error is a document.Err*, then return that rather than any other transient errors.
func retryHandleDocErrs(
	ctx context.Context, retryStrategy retry.Strategy, fn func(ctx context.Context) (bool, error),
) error {
	var err error
	retryErr := retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		var shouldRetry bool
		shouldRetry, err = fn(ctx)
		if !shouldRetry {
			return false, nil
		}
		return true, err
	})
	if retryErr != nil {
		return retryErr
	}
	return err
}

func (s *service) Create(ctx context.Context, ID string, doc interface{}) error {
	return retryHandleDocErrs(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Create(ctx, ID, doc)
		return s.isRetriableError(err), err
	})
}

func (s *service) Set(ctx context.Context, ID string, doc interface{}) error {
	return retryHandleDocErrs(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Set(ctx, ID, doc)
		return s.isRetriableError(err), err
	})
}

func (s *service) Update(ctx context.Context, ID string, updates []document.Update) error {
	return retryHandleDocErrs(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Update(ctx, ID, updates)
		return s.isRetriableError(err), err
	})
}

func (s *service) Get(ctx context.Context, ID string, doc interface{}) error {
	return retryHandleDocErrs(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Get(ctx, ID, doc)
		return s.isRetriableError(err), err
	})
}

func (s *service) Delete(ctx context.Context, ID string) error {
	return retryHandleDocErrs(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Delete(ctx, ID)
		return s.isRetriableError(err), err
	})
}

func (s *service) List(ctx context.Context, filters []document.Filter) (
	it document.Iterator, err error,
) {
	err = retryHandleDocErrs(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		it, err = s.active.List(ctx, filters)
		return s.isRetriableError(err), err
	})
	return
}

func (s *service) isLeader() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc == s.active
}

func (s *service) follow(ctx context.Context, addr net.Addr) (reconnect bool, err error) {
	quitCh := s.quitCh
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()}
	opts = append(opts, grpc.WithDialer(
		func(_ string, _ time.Duration) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, addr.Network(), addr.String())
		},
	))
	// do not override context as we're using it to know when service is closing
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	conn, err := grpc.DialContext(dialCtx, "", opts...)
	cancel()
	if err != nil {
		err = fmt.Errorf("grpc.Dial: %v", err)
		return true, err
	}

	client := new(rpc.Client)
	client.Init(conn)

	// wait until connection is ready to unlock API mutex
loop:
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			s.followFailures = 0 // reset
			// set active svc and unlock API
			s.setActiveAndUnlock(client)
			s.log(log.InfoLevel, "Successfully connected to leader. Unlocking API...")
			break loop
		case connectivity.Connecting, connectivity.Idle:
			conn.WaitForStateChange(ctx, state)
		case connectivity.TransientFailure:
			failureCtx, cancelFn := context.WithTimeout(ctx, transientFailureRecoverTimeout)
			didChange := conn.WaitForStateChange(failureCtx, connectivity.TransientFailure)
			cancelFn()
			if !didChange {
				return true, errors.New("timed out waiting for transient failure to recover")
			}
		case connectivity.Shutdown:
			return true, errors.New("grpc connection state = shutdown")
		default:
			panic(fmt.Sprintf("unknown connection state: %v", state))
		}
	}

	// monitor connection and quit if it exits
	for {
		state := conn.GetState()
		s.log(log.TraceLevel, "Monitoring for state changes. Current: %s", state)
		switch state {
		case connectivity.Ready, connectivity.Idle, connectivity.Connecting:
			if !conn.WaitForStateChange(ctx, state) {
				s.log(log.TraceLevel, "Stopped monitoring for state changes. ctx is canceled")
				return false, nil
			}
		case connectivity.TransientFailure:
			s.log(log.WarnLevel, "connection state is TransientFailure. Waiting for recover with timeout %s",
				transientFailureRecoverTimeout)
			failureCtx, cancelFn := context.WithTimeout(ctx, transientFailureRecoverTimeout)
			didChange := conn.WaitForStateChange(failureCtx, connectivity.TransientFailure)
			cancelFn()
			if !didChange {
				s.log(log.ErrorLevel, "failed to recover from TransientFailure. Reassessing lead/follow position")
				s.mu.Lock() // prevent API calls from making progress while we re-connect
				return true, errors.New("timed out waiting for transient failure to recover")
			}
		case connectivity.Shutdown:
			select {
			case <-quitCh:
				return false, nil
			default:
				s.log(log.ErrorLevel, "connection state is Shutdown")
				s.mu.Lock() // same as above
				return true, errors.New("grpc connection state = shutdown")
			}
		default:
			panic(fmt.Sprintf("unknown connection state: %v", state))
		}
	}
}

func (s *service) setActiveAndUnlock(svc document.Service) {
	s.active = svc
	s.mu.Unlock()
}

func (s *service) lead(ctx context.Context, listener net.Listener) (reconnect bool, err error) {
	server := rpc.NewServer(document.SyncWithLocker(s.svc, &s.mu))
	defer listener.Close()
	defer server.Close()

	done := make(chan error)
	quitCh := s.quitCh
	go func() {
		select {
		case done <- server.Serve(listener):
		case <-quitCh:
		}
	}()

	// set active svc and unlock API
	s.log(log.InfoLevel, "Successfully assumed position of leader. Unlocking API...")
	s.setActiveAndUnlock(s.svc)

	select {
	case <-quitCh:
		return false, nil
	case err := <-done:
		s.mu.Lock()
		return false, err
	}
}

func (s *service) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "firstmover.Service").
		Logf(level, msg, args...)
}

func (s *service) leadOrFollow() {
	quitCh := s.quitCh

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-quitCh
	}()

	err := retry.Retry(ctx, connectRetryStrategy, func(ctx context.Context) (bool, error) {
		cfg := net.ListenConfig{}
		listener, err := cfg.Listen(ctx, "unix", s.lockFile)
		if err == nil {
			retry, err := s.lead(ctx, listener)
			if err == nil || retry {
				return retry, err
			}
			s.log(log.WarnLevel, "Unexpected lead error: %v", err)
			return true, err
		}

		// stop retrying if we don't expect error
		if !strings.Contains(err.Error(), "address already in use") {
			s.log(log.WarnLevel, "Unexpected error while trying to acquire lock %q: %v", s.lockFile, err)
			return false, err
		}

		s.log(log.TraceLevel, "Expected error while trying to acquire lock %q: "+
			"fallback to follow instead: %v", s.lockFile, err)

		addr := net.UnixAddr{Net: "unix", Name: s.lockFile}
		retry, err := s.follow(ctx, &addr)
		if err != nil {
			s.followFailures++
		}
		if s.followFailures >= maxFollowFailures {
			s.followFailures = 0
			s.log(log.WarnLevel, "Unresponsive leader. Removing lock and taking the lead: %v", err)
			_ = os.Remove(s.lockFile)
			return true, err
		}
		if err == nil || retry {
			s.log(log.TraceLevel, "Expected follow error or triggered retry (left %d retries): err=%v",
				maxFollowFailures-s.followFailures, err)
			return retry, err
		}
		s.log(log.WarnLevel, "Unexpected follow error: %v", err)
		return false, err
	})
	select {
	case <-quitCh:
	default:
		s.log(log.ErrorLevel, "Unexpectedly stopped retrying: %v", err)
	}
}

func (s *service) Close() (ret error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.quitCh == nil {
		return
	}
	close(s.quitCh)
	s.quitCh = nil
	if s.active != s.svc {
		if err := s.active.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	if err := s.svc.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return
}
