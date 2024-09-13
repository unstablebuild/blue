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
	"fmt"
	"net"
	"sync"

	"github.com/ernestrc/go-multierror"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document/firstmover/pubsubpb"
	"github.com/unstablebuild/blue/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type pubsub struct {
	pubsubpb.UnimplementedPubSubServer
	mu       sync.Locker
	id       string
	ctx      context.Context
	cancelFn func()
	readyCtx context.Context
	ready    func()
	closed   bool

	// leader only
	subscribers map[string][]subscriber
	leaderConn  *grpc.ClientConn

	// leader and follower
	client        pubsubpb.PubSubClient
	clientStreams map[string]pubsubpb.PubSub_ReceiveClient // stream cache
}

type subscriber struct {
	errors chan error
	stream pubsubpb.PubSub_ReceiveServer
}

func (p *pubsub) init() {
	p.readyCtx, p.ready = context.WithCancel(context.Background())
}

func (p *pubsub) reset() {
	p.clientStreams = make(map[string]pubsubpb.PubSub_ReceiveClient)
	p.subscribers = make(map[string][]subscriber)
	p.id = uuid.New().String()
	p.ctx, p.cancelFn = context.WithCancel(context.Background())
	p.closed = false
}

// initLeader assumes this pubsub server has already been registered.
// This cannot be done here because RegisterPubSubServer cannot be
// called after Serve is called, but the dial below will always fail
// if Serve has not been called yet.
func (p *pubsub) initLeader(
	ctx context.Context, srv *grpc.Server, listener net.Listener,
) error {
	_ = p.Close()
	p.reset()
	p.ready()

	// to massively simplify streams implementation,
	// publish/subscribe as a leader also act as a client of the pubsub server.
	addr := listener.Addr()
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
	conn, err := grpc.DialContext(ctx, "", opts...)
	if err != nil {
		return fmt.Errorf("dial leader server: %v", err)
	}

	p.client = pubsubpb.NewPubSubClient(conn)
	p.leaderConn = conn
	p.log(log.DebugLevel, "initialized pubsub instance as leader")
	return nil
}

func (p *pubsub) initFollower(conn grpc.ClientConnInterface) {
	_ = p.Close()
	p.reset()
	p.client = pubsubpb.NewPubSubClient(conn)
	p.log(log.DebugLevel, "initialized pubsub instance as follower")
}

func (p *pubsub) publish(
	ctx context.Context,
	topic string, msg []byte,
) error {
	p.mu.Lock()
	client := p.client
	p.mu.Unlock()
	req := pubsubpb.PublishRequest{
		Topic:  topic,
		Data:   msg,
		Sender: p.id,
	}
	p.log(log.TraceLevel, "client is publishing message")
	_, err := client.Publish(ctx, &req)
	if err != nil {
		return err
	}

	return nil
}

func (p *pubsub) subscribe(
	ctx context.Context, topic string,
) (pubsubpb.PubSub_ReceiveClient, error) {
	p.mu.Lock()
	stream, ok := p.clientStreams[topic]
	// if topic stream doesn't exist, create a new one
	if !ok {
		req := pubsubpb.ReceiveRequest{
			Topic: topic,
		}
		var err error
		stream, err = p.client.Receive(ctx, &req)
		if err != nil {
			return nil, err
		}
		p.clientStreams[topic] = stream
		p.mu.Unlock()
		// blocks until server has sent header and so
		// connection is fully established
		md, err := stream.Header()
		if err != nil {
			return nil, err
		}

		p.log(log.DebugLevel, "subscribed to topic %s, metadata: %+v", topic, md)
	} else {
		p.mu.Unlock()
	}

	return stream, nil
}

func (p *pubsub) receive(
	ctx context.Context, topic string,
) ([]byte, error) {
	stream, err := p.subscribe(ctx, topic)
	if err != nil {
		return nil, err
	}

	for {
		p.log(log.TraceLevel, "client is waiting to receive a message on stream %p", stream)
		msg, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		// when we publish as a leader, we should not receive
		// these messages on the next call to Receive.
		if msg.GetSender() == p.id {
			continue
		}
		return msg.GetData(), nil
	}
}

func (p *pubsub) Publish(
	ctx context.Context, req *pubsubpb.PublishRequest,
) (resp *pubsubpb.PublishResponse, err error) {
	<-p.readyCtx.Done()

	topic := req.GetTopic()
	msg := req.GetData()

	p.mu.Lock()
	subscribers := p.subscribers[topic]
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil, status.Errorf(codes.Aborted, "closed")
	}

	p.log(log.TraceLevel, "server is broadcasting message for topic %q: subscribers: %d",
		topic, len(subscribers))

	for _, sub := range subscribers {
		var req pubsubpb.ReceiveResponse
		req.Data = msg
		if serr := sub.stream.Send(&req); serr != nil {
			// cancel offending stream, but also return
			// an error to this rpc so we provide at least once semantics
			select {
			case sub.errors <- fmt.Errorf("stream send: %w", serr):
			default:
			}
			err = multierror.Append(err, serr)
		}
	}

	p.log(log.TraceLevel, "server is done broadcasting message for topic %q to %d subscribers",
		topic, len(subscribers))

	if err != nil {
		return nil, err
	}
	return new(pubsubpb.PublishResponse), nil
}

func (p *pubsub) Receive(
	req *pubsubpb.ReceiveRequest, srv pubsubpb.PubSub_ReceiveServer,
) error {
	<-p.readyCtx.Done()

	topic := req.GetTopic()
	errors := make(chan error)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return status.Errorf(codes.Aborted, "closed")
	}
	p.subscribers[topic] = append(p.subscribers[topic],
		subscriber{
			errors: errors,
			stream: srv,
		},
	)
	p.mu.Unlock()

	// header is waited upon by client to guarantee that
	// the subscriber will be found, if Receive is followed by fast
	// subsequent calls to Publish.
	md := metadata.New(make(map[string]string))
	md.Append("ID", p.id)
	err := srv.SendHeader(md)
	if err != nil {
		return fmt.Errorf("rpc send header: %w", err)
	}

	// remove ch when this receive stream is done
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		for i, c := range p.subscribers[topic] {
			if c.errors == errors {
				copy(p.subscribers[topic][i:], p.subscribers[topic][i+1:])
				p.subscribers[topic] = p.subscribers[topic][:len(p.subscribers[topic])-1]
				return
			}
		}
	}()

	select {
	case err := <-errors:
		return err
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-srv.Context().Done():
		return nil
	}
}

func (p *pubsub) Close() (err error) {
	if p.cancelFn != nil {
		p.cancelFn()
	}

	if p.leaderConn != nil {
		err = p.leaderConn.Close()
	}

	p.subscribers = nil
	p.clientStreams = nil
	p.closed = true

	return err
}

func (s *pubsub) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{
		logging.KeyClass: "firstmover.pubsub",
		"address":        fmt.Sprintf("%p", s),
	}).Logf(level, msg, args...)
}
