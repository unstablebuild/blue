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

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/encoding/bson"
	"github.com/unstablebuild/blue/encoding/json"
	"github.com/unstablebuild/blue/encoding/toml"
	"github.com/stretchr/testify/require"
)

func TestPartitionService(t *testing.T) {
	tsuite := []struct {
		encoding  string
		marshaler encoding.Marshaler
	}{
		{"bson", bson.Marshaler()},
		{"json", json.Marshaler()},
		{"toml", toml.Marshaler()},
	}
	for _, tcase := range tsuite {
		t.Run("a single, default partition", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				TestDocumentService(t, func(t *testing.T) document.Service {
					other := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
					return document.WithPartition(other, "default")
				})
			})
		})

		t.Run("multiple partitions", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				other := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
				var i int
				TestDocumentService(t, func(t *testing.T) document.Service {
					i++
					return document.WithPartition(other, fmt.Sprintf("%dth", i))
				})
			})
		})
	}

	// simple tests to make debuggin easier, but complete test is above in "multiple partitions"
	t.Run("records created by one are not seend by another", func(t *testing.T) {
		other := document.NewInMemoryService()
		one := document.WithPartition(other, "one")
		two := document.WithPartition(other, "two")

		type myStruct struct {
			A string
		}
		ctx := context.Background()
		require.NoError(t, one.Create(ctx, "myID", myStruct{A: "a"}))
		require.NoError(t, two.Create(ctx, "myID", myStruct{A: "a"}))

		it, err := one.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, 1)

		it2, err := two.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it2, 1)
	})
}
