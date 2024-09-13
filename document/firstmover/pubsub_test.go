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

func makeLeaderFollowerPair(t *testing.T, nfollowers int) (*Service, []*Service) {
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
		followers = append(followers, New(svc, lockFile, cfg))
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

func TestPubSub(t *testing.T) {
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
