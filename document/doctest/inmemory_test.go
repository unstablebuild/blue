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

package doctest

import (
	"testing"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/encoding/bson"
	"github.com/unstablebuild/blue/encoding/json"
	"github.com/unstablebuild/blue/encoding/toml"
)

func TestInMemoryService(t *testing.T) {
	tsuite := []struct {
		encoding  string
		marshaler encoding.Marshaler
	}{
		{"bson", bson.Marshaler()},
		{"json", json.Marshaler()},
		{"toml", toml.Marshaler()},
		// NOTE: yaml passes all tests except the ones with encoding of
		// numerical values. It should never be used as a storage format anyway.
		// {"yaml", yaml.Marshaler()},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.encoding, func(t *testing.T) {
			TestDocumentService(t, func(t *testing.T) document.Service {
				return document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
			})
			// NOTE: time preconditions don't quite work in json
			// or toml due to lossy time marshalling
			if tcase.encoding == "json" || tcase.encoding == "toml" {
				return
			}
			TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
				return document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
			})
		})
	}
}
