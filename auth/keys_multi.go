// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import "context"

// CombineKeys combines a set of Keys, tipically used in calls to Verify.
// Sign will return the key in k.
func CombineKeys(k Keys, extra ...Keys) Keys {
	keys := make([]Keys, 0, len(extra)+1)
	keys = append(keys, k)
	keys = append(keys, extra...)
	return multiKeys{keys: keys}
}

type multiKeys struct {
	keys []Keys
}

func (m multiKeys) Sign(ctx context.Context) (Key, error) {
	return m.keys[0].Sign(ctx)
}

func (m multiKeys) Verify(ctx context.Context) (ret []Key, err error) {
	for _, keys := range m.keys {
		other, err := keys.Verify(ctx)
		if err != nil {
			return nil, err
		}
		ret = append(ret, other...)
	}
	return
}
