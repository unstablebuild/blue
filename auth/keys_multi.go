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
