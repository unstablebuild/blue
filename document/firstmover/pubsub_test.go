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
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/retry"
)

func TestPubSub(t *testing.T) {
	t.Run("Close called more than once doesn't block or panic", func(t *testing.T) {
		svc, _ := makeLeaderFollowerPair(t, 0)
		assert.NotPanics(t, func() {
			require.NoError(t, svc.Close())
			require.NoError(t, svc.Close())
			require.NoError(t, svc.Close())
		})
	})

	t.Run("leader subscribe more than once returns an error ", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		require.NoError(t, leader.Subscribe(context.Background(), "coffee"))
		require.Error(t, leader.Subscribe(context.Background(), "coffee"))
		t.Cleanup(func() {
			_ = leader.Close()
			for _, follower := range followers {
				_ = follower.Close()
			}
		})
	})

	t.Run("follower subscribe more than once returns an error ", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		require.NoError(t, followers[0].Subscribe(context.Background(), "coffee"))
		require.Error(t, followers[0].Subscribe(context.Background(), "coffee"))
		t.Cleanup(func() {
			_ = leader.Close()
			for _, follower := range followers {
				_ = follower.Close()
			}
		})
	})

	t.Run("nodes receive ErrMessageTooLarge if "+
		"trying to publish a message larger than MaxMessageSize", func(t *testing.T) {
		leader, follower := makeLeaderFollowerPair(t, 1)

		ctx := context.Background()
		err := leader.Publish(ctx, "1234", make([]byte, leader.cfg.MaxMessageSize*2))
		require.Equal(t, ErrMessageTooLarge, err)

		err = follower[0].Publish(ctx, "1234", make([]byte, leader.cfg.MaxMessageSize*2))
		require.Equal(t, ErrMessageTooLarge, err)

		assert.NoError(t, leader.Close())
		assert.NoError(t, follower[0].Close())
	})

	t.Run("leader is able to publish to a topic and "+
		"follower receive a message from a topic", func(t *testing.T) {
		leader, follower := makeLeaderFollowerPair(t, 1)

		publishAndReceive(t, leader, follower[0], 1)

		assert.NoError(t, leader.Close())
		assert.NoError(t, follower[0].Close())
	})

	t.Run("follower is able to publish to a topic and "+
		"leader receive message from a topic", func(t *testing.T) {
		leader, follower := makeLeaderFollowerPair(t, 1)

		publishAndReceive(t, follower[0], leader, 1)

		assert.NoError(t, leader.Close())
		assert.NoError(t, follower[0].Close())
	})

	t.Run("leader is able to publish to multiple topics and "+
		"follower receive messages from multiple topics", func(t *testing.T) {
		leader, follower := makeLeaderFollowerPair(t, 1)

		publishAndReceive(t, leader, follower[0], 100)

		assert.NoError(t, leader.Close())
		assert.NoError(t, follower[0].Close())
	})

	t.Run("follower is able to publish to multiple topics and "+
		"leader receive messages from multiple topics", func(t *testing.T) {
		leader, follower := makeLeaderFollowerPair(t, 1)

		publishAndReceive(t, follower[0], leader, 100)

		assert.NoError(t, leader.Close())
		assert.NoError(t, follower[0].Close())
	})

	t.Run("follower is able to publish to another follower ", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 2)
		follower1 := followers[0]
		follower2 := followers[1]

		publishAndReceive(t, follower1, follower2, 1)

		assert.NoError(t, leader.Close())
		assert.NoError(t, follower1.Close())
		assert.NoError(t, follower2.Close())
	})

	t.Run("follower is able to publish to leader after another follower dies", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 2)
		follower1 := followers[0]
		follower2 := followers[1]
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// force create a stream channel
			_, _ = follower2.Receive(ctx, "0")
		}()
		require.NoError(t, follower2.Close())
		cancel()

		publishAndReceive(t, follower1, leader, 1)

		assert.NoError(t, leader.Close())
		assert.NoError(t, follower1.Close())
	})

	t.Run("follower takes the lead while trying to publish or receive", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 2)
		follower1 := followers[0]
		follower2 := followers[1]
		require.NoError(t, leader.Close())

		publishAndReceive(t, follower1, follower2, 1)
		publishAndReceive(t, follower2, follower1, 1)

		assert.NoError(t, follower1.Close())
		assert.NoError(t, follower2.Close())
	})

	t.Run("any node is able to publish to multiple nodes", func(t *testing.T) {
		suite := []struct {
			indexPub int
			n        int
		}{
			{0, 2},
			{1, 2},
			{0, 3},
			{1, 3},
			{2, 3},
			{0, 5},
			{1, 5},
			{2, 5},
			{3, 5},
			{4, 5},
			{0, 100},
			{50, 100},
			{99, 100},
		}
		for _, test := range suite {
			desc := fmt.Sprintf("%d node is able to publish to the rest of nodes (n=%d)",
				test.indexPub, test.n)
			t.Run(desc, func(t *testing.T) {
				leader, followers := makeLeaderFollowerPair(t, test.n)
				nodes := append(followers, leader)
				ctx := context.Background()
				topic := "1234"

				publisher := nodes[test.indexPub]
				rest := make([]*Service, 0)
				rest = append(rest, nodes[:test.indexPub]...)
				rest = append(rest, nodes[test.indexPub+1:]...)

				for _, node := range rest {
					require.NoError(t, node.Subscribe(ctx, topic))
				}

				require.NoError(t, publisher.Publish(ctx, topic, []byte("block")))

				for _, node := range rest {
					data, err := node.Receive(ctx, topic)
					require.NoError(t, err)
					assert.Equal(t, "block", string(data))
				}

				for _, node := range nodes {
					assert.NoError(t, node.Close())
				}
			})
		}
	})

	t.Run("receive after Close should not block, and instead return an error", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 5)
		nodes := append(followers, leader)
		ctx := context.Background()
		topic := "1234"

		indexPub := 1
		publisher := nodes[indexPub]
		rest := make([]*Service, 0)
		rest = append(rest, nodes[:indexPub]...)
		rest = append(rest, nodes[indexPub+1:]...)

		for _, node := range rest {
			require.NoError(t, node.Subscribe(ctx, topic))
		}

		require.NoError(t, publisher.Publish(ctx, topic, []byte("block")))

		for _, node := range nodes[2:] { // 0 is last receiver, 1 is publisher
			assert.NoError(t, node.Close())
			_, err := node.Receive(ctx, topic)
			require.Error(t, err)

			// double close is no-op
			assert.NoError(t, node.Close())
		}

		data, err := nodes[0].Receive(ctx, topic)
		require.NoError(t, err)
		assert.Equal(t, "block", string(data))

		assert.NoError(t, nodes[0].Close())
	})

	t.Run("leader/follower Receive times out, no subscribe", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		topic := "1234"

		ctx := context.Background()
		for _, node := range []*Service{leader, followers[0]} {
			ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			_, err := node.Receive(ctx, topic)
			require.Error(t, err)
			cancel()
		}

		assert.NoError(t, leader.Close())
		assert.NoError(t, followers[0].Close())
	})

	t.Run("Receive with implicit subscribe times out, subscription is not cancelled", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		topic := "1234"

		ctx := context.Background()
		for i, node := range []*Service{leader, followers[0]} {
			ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			_, err := node.Receive(ctx, topic)
			require.Error(t, err)
			cancel()

			ctx = context.Background()
			if i == 0 {
				require.NoError(t, followers[0].Publish(ctx, topic, []byte("block")))
			} else {
				require.NoError(t, leader.Publish(ctx, topic, []byte("block")))
			}

			data, err := node.Receive(ctx, topic)
			require.NoError(t, err)
			assert.Equal(t, "block", string(data))
		}

		assert.NoError(t, leader.Close())
		assert.NoError(t, followers[0].Close())
	})

	t.Run("leader/follower Receive times out, prior Subscribe", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		topic := "1234"

		ctx := context.Background()
		for _, node := range []*Service{leader, followers[0]} {
			err := node.Subscribe(ctx, topic)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			_, err = node.Receive(ctx, topic)
			require.Error(t, err)
			cancel()
		}

		assert.NoError(t, leader.Close())
		assert.NoError(t, followers[0].Close())
	})

	t.Run("leader/follower Receive times out, then publish, then receive succeeds", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		topic := "1234"

		nodes := []*Service{leader, followers[0]}
		ctx := context.Background()
		for i, node := range nodes {
			err := node.Subscribe(ctx, topic)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			_, err = node.Receive(ctx, topic)
			require.Error(t, err)
			cancel()

			var publisher *Service
			if i == 0 {
				publisher = nodes[1]
			} else {
				publisher = nodes[0]
			}
			require.NoError(t, publisher.Publish(context.Background(), topic, []byte("block")))

			data, err := node.Receive(context.Background(), topic)
			require.NoError(t, err)
			assert.Equal(t, "block", string(data))
		}

		assert.NoError(t, leader.Close())
		assert.NoError(t, followers[0].Close())
	})

	t.Run("client re-connect", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 2)
		topic := "1234"

		ctx := context.Background()
		err := followers[0].Subscribe(ctx, topic)
		require.NoError(t, err)

		err = followers[1].Subscribe(ctx, topic)
		require.NoError(t, err)

		require.NoError(t, leader.Close())

		require.NoError(t, followers[1].Publish(context.Background(), topic, []byte("block")))

		data, err := followers[0].Receive(context.Background(), topic)
		require.NoError(t, err)
		assert.Equal(t, "block", string(data))

		assert.NoError(t, followers[0].Close())
		assert.NoError(t, followers[1].Close())
	})

	t.Run("receive is able to receive messages across leader/follower handoffs", func(t *testing.T) {
		// change the follower's listen lockFile to be a regular file
		// so we ensure that across leader re-elections, this instance
		// never becomes the leader.
		lockFile, err := os.CreateTemp("", "")
		require.NoError(t, err)
		_, err = lockFile.WriteString("abc")
		require.NoError(t, err)
		require.NoError(t, lockFile.Close())

		leader, followers := makeLeaderFollowerPairLockFileListen(t, 1, lockFile.Name())
		topic := "1234"
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		err = followers[0].Subscribe(ctx, topic)
		require.NoError(t, err)

		const n = 2
		var data [n][]byte
		var errs [n]error
		var wg sync.WaitGroup
		wg.Add(n)
		go func() {
			for i := 0; i < n; i++ {
				data[i], errs[i] = followers[0].Receive(ctx, topic)
				wg.Done()
			}
		}()

		require.NoError(t, leader.Close())
		for i := 0; i < n; i++ {
			newLeader := New(leader.svc, leader.lockFileListen, leader.cfg)
			for {
				// NOTE: if Publish below is published before follower is able
				// to connect, then the message is lost.
				err := followers[0].Get(ctx, "abc", make(map[string]interface{}))
				if errors.Is(err, document.ErrNotFound) {
					break
				}
			}
			require.NoError(t, newLeader.Publish(ctx, topic, []byte("block")))
			newLeader.Close()
		}

		wg.Wait()
		for i := 0; i < n; i++ {
			data, err := data[i], errs[i]
			require.NoError(t, err)
			assert.Equal(t, "block", string(data))
		}

		assert.NoError(t, followers[0].Close())
	})

	t.Run("extreme concurrency of leaders and followers", func(t *testing.T) {
		const n, m = 100, 50
		cfg := testConfig()

		lockFile := makeTempLockFile(t)
		svc := document.NewInMemoryService()

		instances := make([]*Service, 0, n)

		for i := 0; i < n-1; i++ {
			instance := New(svc, lockFile, cfg)
			// override strategy so we guarantee that we do not hit the limit
			instance.retryStrategy = retry.CombinedStrategy(
				retry.SequentialStrategy(2*time.Millisecond),
				retry.LimitStrategy(m*2),
			)
			_ = instance.Get(context.Background(), lockFile, nil)
			instances = append(instances, instance)
		}
		instance1 := instances[len(instances)-1]
		instance2 := instances[len(instances)-2]

		go func() {
			// kill leaders at a cadence manageable by the retry strategy above
			for i := 0; i < n-2; i++ { // always leave two fully operating
				time.Sleep(cfg.DialTimeout + cfg.ConnectRetryCadence)
				for _, instance := range instances {
					if instance.IsLeader() && instance != instance1 && instance != instance2 {
						instance.Close()
						break
					}
				}
			}
		}()
		publishAndReceive(t, instance1, instance2, m)
	})
}

func makeLeaderFollowerPair(t *testing.T, nfollowers int) (*Service, []*Service) {
	return makeLeaderFollowerPairLockFileListen(t, nfollowers, "")
}

func makeLeaderFollowerPairLockFileListen(
	t *testing.T, nfollowers int,
	lockFileListen string,
) (*Service, []*Service) {
	lockFile := makeTempLockFile(t)
	cfg := testConfig()
	cfg.Marshaler = doctoml.Marshaler()
	svc := document.NewInMemoryServiceWithMarshaler(cfg.Marshaler)
	leader := New(svc, lockFile, cfg)
	// ensure leader is available
	var temp testStruct
	err := leader.Get(context.Background(), lockFile, &temp)
	require.Equal(t, document.ErrNotFound, err)
	var followers []*Service
	for i := 0; i < nfollowers; i++ {
		follower := new(Service)
		follower.lockFileListen = lockFileListen
		follower.Init(svc, lockFile, cfg)
		followers = append(followers, follower)
	}
	return leader, followers
}

func publishAndReceive(t *testing.T, sender, receiver *Service, n int) {
	ctx := context.Background()

	var done, ready sync.WaitGroup
	actualMsg := make([][]byte, n)
	actualErr := make([]error, n)

	done.Add(n)
	ready.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer done.Done()
			actualErr[i] = receiver.Subscribe(ctx, strconv.Itoa(i))
			ready.Done()
			if actualErr[i] != nil {
				return
			}
			actualMsg[i], actualErr[i] = receiver.Receive(
				ctx, strconv.Itoa(i))
		}(i)
	}

	ready.Wait()
	// unfortunately it's impossible to truly hook into the internal stream Recv
	// and know for sure that there's an actual subscriber ready for that
	for i := 0; i < n; i++ {
		err := sender.Publish(ctx, strconv.Itoa(i), []byte(strconv.Itoa(i)))
		require.NoError(t, err)
	}
	done.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, actualErr[i], i)
		assert.Equal(t, strconv.Itoa(i), string(actualMsg[i]), i)
	}
}
