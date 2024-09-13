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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/ernestrc/go-multierror"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docrpc"
	"github.com/unstablebuild/blue/document/docrpc/docpb"
	"github.com/unstablebuild/blue/document/firstmover/pubsubpb"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Service is a document.Service that either acquires a lock
// by creating a unix socket at lockFile and exposes svc
// to RPC clients or if it fails to acquire lock, it will connect
// to the current leader via lockFile.
//
// If the leader is closed after this Service connects to it,
// or it stops responding for more than a specificed timeout,
// all running Services returned will race to re-acquire the lock and
// act as the new leader.
//
// This Service also exposes pub/sub capabilities with at least once
// semantics.
type Service struct {
	pubsub   *pubsub
	mu       sync.Mutex
	svc      document.Service
	lockFile string

	cfg                  Config
	maxFollowFailures    int
	retryStrategy        retry.Strategy
	connectRetryStrategy retry.Strategy

	closed      bool
	quitCh      chan struct{}
	closeWaitCh chan struct{}

	followFailures int
	active         document.Service
}

// New allocates storage for a new Service and initializes it.
func New(svc document.Service, lockFile string, cfg Config) *Service {
	ret := new(Service)
	ret.Init(svc, lockFile, cfg)
	return ret
}

// Init initializes this Service to lead or follow, depending on whether
// lockFile has already been created or not.
//
// It is highly recommended to use DefaultConfig to build a sane Config.
func (s *Service) Init(svc document.Service, lockFile string, cfg Config) {
	if cfg.Marshaler == nil {
		panic("empty Marshaler in config")
	}
	s.svc = svc
	s.lockFile = lockFile
	s.pubsub = new(pubsub)
	s.pubsub.mu = &s.mu

	s.cfg = cfg
	s.maxFollowFailures = int(cfg.TimeToCoup / (cfg.DialTimeout + cfg.ConnectRetryCadence))
	s.retryStrategy = retry.CombinedStrategy(
		retry.SequentialStrategy(cfg.MethodRetryCadence),
		retry.LimitStrategy(uint(cfg.TimeToCoup/cfg.MethodRetryCadence*2)),
	)
	s.connectRetryStrategy = retry.SequentialStrategy(cfg.ConnectRetryCadence)

	s.quitCh = make(chan struct{})
	s.closeWaitCh = make(chan struct{})

	s.mu.Lock()
	go s.leadOrFollow()
}

// Create satisfies document.Service.
func (s *Service) Create(ctx context.Context, ID string, doc interface{}) error {
	return retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Create(ctx, ID, doc)
		return s.isRetriableError(err), err
	})
}

// Set satisfies document.Service.
func (s *Service) Set(ctx context.Context, ID string, doc interface{}) error {
	return retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Set(ctx, ID, doc)
		return s.isRetriableError(err), err
	})
}

// Update satisfies document.Service.
func (s *Service) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	return retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Update(ctx, ID, updates, preconds...)
		return s.isRetriableError(err), err
	})
}

// Get satisfies document.Service.
func (s *Service) Get(ctx context.Context, ID string, doc interface{}) error {
	return retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Get(ctx, ID, doc)
		return s.isRetriableError(err), err
	})
}

// Delete satisfies document.Service.
func (s *Service) Delete(ctx context.Context, ID string) error {
	return retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.active.Delete(ctx, ID)
		return s.isRetriableError(err), err
	})
}

// List satisfies document.Service.
func (s *Service) List(ctx context.Context, filters []document.Filter) (
	it document.Iterator, err error,
) {
	err = retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		it, err = s.active.List(ctx, filters)
		return s.isRetriableError(err), err
	})
	return
}

// Publish publishes an arbitrary message to the given topic.
// It will be received by all subscribers of this topic.
func (s *Service) Publish(
	ctx context.Context, topic string, msg []byte,
) error {
	return retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		err := s.pubsub.publish(ctx, topic, msg)
		return s.isRetriableError(err), err
	})
}

// Subscribe creates a subscription created to the given topic,
// ensuring that future messages published are buffered for the next calls
// to Receive.
func (s *Service) Subscribe(
	ctx context.Context, topic string,
) error {
	return retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		_, err := s.pubsub.subscribe(ctx, topic)
		return s.isRetriableError(err), err
	})
}

// Receive returns the next message published to the given topic,
// or blocks until a message is available.
//
// Under the hood a subscription is created so messages
// between calls to Receive are never lost.
func (s *Service) Receive(
	ctx context.Context, topic string,
) (data []byte, err error) {
	err = retryHandleDocErrs(ctx, s.retryStrategy, func(ctx context.Context) (bool, error) {
		data, err = s.pubsub.receive(ctx, topic)
		return s.isRetriableError(err), err
	})
	return
}

// Close closes all resources associated with this Service.
func (s *Service) Close() (ret error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.quitCh)
	if s.active != s.svc {
		if err := s.active.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	if err := s.svc.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}

	if err := s.pubsub.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	// wait until we're sure that lock has been removed
	// if we're the leader
	<-s.closeWaitCh
	return
}

// IsLeader returns whether this instance is the leader of the system.
func (s *Service) IsLeader() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc == s.active
}

func (s *Service) follow(ctx context.Context, addr net.Addr) (bool, error) {
	quitCh := s.quitCh
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}
	opts = append(opts, grpc.WithContextDialer(
		func(ctx context.Context, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, addr.Network(), addr.String())
		},
	))
	// do not override context as we're using it to know when Service is closing
	dialCtx, cancel := context.WithTimeout(ctx, s.cfg.DialTimeout)
	conn, err := grpc.DialContext(dialCtx, "", opts...)
	cancel()
	if err != nil {
		// any Dial errors should always be retried. Any socket specific errors
		// that are not expected, and therefore would trigger a full halt will
		// be handled by the leader erro handling logic.
		return true, err
	}

	client := new(docrpc.Client)
	client.Init(conn, s.cfg.Marshaler)

	// wait until connection is ready to unlock API mutex
loop:
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			s.followFailures = 0 // reset
			s.pubsub.init()
			s.pubsub.initFollower(conn)
			// set active svc and unlock API
			s.setActiveAndUnlock(client)
			s.log(log.DebugLevel, "Successfully connected to leader. Unlocking API...")
			break loop
		case connectivity.Connecting, connectivity.Idle:
			conn.WaitForStateChange(ctx, state)
		case connectivity.TransientFailure:
			failureCtx, cancelFn := context.WithTimeout(ctx, s.cfg.TransientFailureRecoverTimeout)
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
				s.cfg.TransientFailureRecoverTimeout)
			failureCtx, cancelFn := context.WithTimeout(ctx, s.cfg.TransientFailureRecoverTimeout)
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

func (s *Service) setActiveAndUnlock(svc document.Service) {
	s.active = svc
	s.mu.Unlock()
}

func (s *Service) lead(ctx context.Context, listener net.Listener) (reconnect bool, err error) {
	server := docrpc.NewServer(document.SyncWithLocker(s.svc, &s.mu), s.cfg.Marshaler)
	defer listener.Close()

	gsrv := grpc.NewServer()
	defer gsrv.Stop()

	s.pubsub.init()
	docpb.RegisterDocumentStoreServer(gsrv, server)
	pubsubpb.RegisterPubSubServer(gsrv, s.pubsub)

	done := make(chan error)
	ready := make(chan struct{})
	quitCh := s.quitCh
	go func() {
		select {
		case done <- gsrv.Serve(&unlockListener{ready: ready, root: listener}):
		case <-quitCh:
		}
	}()

	// wait for grpcserver to be listening
	// before we initialize pubsub as leader
	select {
	case <-ready:
	case <-ctx.Done():
		return true, ctx.Err()
	}

	if err := s.pubsub.initLeader(ctx, gsrv, listener); err != nil {
		return false, fmt.Errorf("set pubsub leader: %w", err)
	}

	// set active svc and unlock API
	s.log(log.DebugLevel, "Successfully assumed position of leader. Unlocking API...")
	s.setActiveAndUnlock(s.svc)

	select {
	case <-quitCh:
		return false, nil
	case err := <-done:
		s.mu.Lock()
		return false, err
	}
}

func (s *Service) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "firstmover.Service").
		Logf(level, msg, args...)
}

func (s *Service) leadOrFollow() {
	quitCh := s.quitCh
	defer close(s.closeWaitCh)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-quitCh
	}()

	err := retry.Retry(ctx, s.connectRetryStrategy, func(ctx context.Context) (bool, error) {
		var cfg net.ListenConfig
		listener, err := cfg.Listen(ctx, "unix", s.lockFile)
		if err == nil {
			retry, err := s.lead(ctx, listener)
			if err == nil || retry {
				return retry, err
			}
			s.log(log.WarnLevel, "Unexpected lead error: %v", err)
			return true, err
		}

		if errors.Is(err, syscall.ENOENT) { // a component of the path does not exist
			mkdirErr := os.MkdirAll(filepath.Dir(s.lockFile), 0766)
			if mkdirErr != nil {
				s.log(log.WarnLevel, "create lock dir: %v", mkdirErr)
				return false, multierror.Append(err, mkdirErr)
			}
			return true, err
		}

		// depending on whether the error is a bind error or other we need to
		// wrap syscall errors and their os counterparts
		if !errors.Is(err, syscall.EADDRINUSE) && // address already in use
			!errors.Is(err, os.ErrExist) && // file already exists
			!errors.Is(err, os.ErrInvalid) && // socket already bound to an address
			!errors.Is(err, syscall.EINVAL) { // socket already bound to an address
			s.log(log.WarnLevel, "Unexpected error while trying to "+
				"acquire lock %q: %v", s.lockFile, err)
			return false, err
		}

		s.log(log.TraceLevel, "Expected error while trying to acquire lock %q: "+
			"fallback to follow instead: %v", s.lockFile, err)

		addr := net.UnixAddr{Net: "unix", Name: s.lockFile}
		retry, err := s.follow(ctx, &addr)
		if err != nil {
			s.followFailures++
		}
		if s.followFailures >= s.maxFollowFailures {
			s.followFailures = 0
			s.log(log.WarnLevel, "Unresponsive leader. Removing lock and taking the lead: %v", err)
			_ = os.Remove(s.lockFile)
			return true, err
		}
		if err == nil || retry {
			s.log(log.TraceLevel, "Service is closing or expected follow error (left %d retries): err=%v",
				s.maxFollowFailures-s.followFailures, err)
			return retry, err
		}
		s.log(log.WarnLevel, "Unexpected follow error: %v", err)
		return false, err
	})
	select {
	case <-quitCh:
	default:
		s.log(log.ErrorLevel, "Unexpectedly stopped retrying: %v", err)
		s.mu.Unlock() // unblock API as we 're not trying to reconnect again
	}
}

func (s *Service) isRetriableError(err error) bool {
	// for readibility's sake, do not coalesce all branches into a boolean value
	if err == nil {
		return false
	}

	stat := status.Convert(err)
	if s.svc != s.active && s.cfg.CloseError != nil &&
		strings.Contains(stat.Message(), s.cfg.CloseError.Error()) {
		return true
	}

	c := stat.Code()
	return strings.Contains(stat.Message(), "connection error") ||
		strings.Contains(stat.Message(), "EOF") ||
		c == codes.Unavailable || c == codes.DeadlineExceeded || c == codes.Aborted
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
			// return nil so retryErr is nil and we know that we need to
			// return original error
			return shouldRetry, nil
		}
		return shouldRetry, err
	})
	if retryErr != nil {
		return retryErr
	}
	return err
}

var _ net.Listener = (*unlockListener)(nil)

type unlockListener struct {
	root        net.Listener
	ready       chan struct{}
	readyClosed atomic.Bool
}

// Accept signals that the underlying server is ready
func (u *unlockListener) Accept() (net.Conn, error) {
	if u.readyClosed.CompareAndSwap(false, true) {
		close(u.ready)
	}
	return u.root.Accept()
}

func (u *unlockListener) Close() error {
	return u.root.Close()
}

// Addr returns the listener's network address.
func (u *unlockListener) Addr() net.Addr {
	return u.root.Addr()
}
