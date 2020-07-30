package document

import (
	"errors"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func encode(data interface{}, addCreatedAt bool) []byte {
	var ir map[string]interface{}
	err := mapstructure.Decode(data, &ir)
	if err != nil {
		// errors are returned only when input and output types are unexpected
		// or have unexpected fields: panic so we catch early
		panic(err)
	}

	if addCreatedAt {
		ir[DefaultCreatedAtField] = time.Now()
		ir[DefaultUpdatedAtField] = time.Now()
	}

	b, err := bson.Marshal(ir)
	if err != nil {
		panic(err)
	}
	return b
}

func isEncodeable(doc interface{}) bool {
	v := reflect.ValueOf(doc)
	return (v.Kind() == reflect.Ptr || v.Kind() == reflect.Map) && !v.IsNil()
}

func safeDecode(rcv interface{}, raw []byte) error {
	if !isEncodeable(rcv) {
		return errors.New("receiver is not a pointer and not a map or is nil")
	}
	decode(rcv, raw)
	return nil
}

func decode(rcv interface{}, raw []byte) {
	var ir map[string]interface{}
	err := bson.Unmarshal(raw, &ir)
	if err != nil {
		panic(err)
	}

	err = mapstructure.Decode(ir, rcv)
	if err != nil {
		panic(err)
	}
}

func updateProto(updates []Update, proto map[string]interface{}) {

	for _, update := range updates {
		if len(update.FieldPath) == 0 {
			panic("empty field path")
		}
		if update.FieldPath[0] == DefaultUpdatedAtField {
			continue
		}
		updateField(proto, update)
	}

	updateField(proto, Update{
		FieldPath: []string{DefaultUpdatedAtField},
		Value:     time.Now(),
	})
}

func updateField(proto map[string]interface{}, update Update) {
	if len(update.FieldPath) == 1 {
		proto[update.FieldPath[0]] = update.Value
		return
	}

	field, exist := proto[update.FieldPath[0]]
	if !exist {
		// surprisingly, FireStore does not return an error and also
		// it doesn't add the extra fields. Since we are emulating
		// the same behaviour, just return silently instead of
		// creating the attribute path.
		return
	}

	m, ok := field.(map[string]interface{})
	if !ok {
		panic("corrupted record: field node is not a map")
	}

	update.FieldPath = update.FieldPath[1:]
	updateField(m, update)
}

func derefCreateValue(data reflect.Value) (interface{}, error) {
	for {
		switch data.Kind() {
		case reflect.Struct, reflect.Map:
			return data.Interface(), nil
		case reflect.Ptr:
			data = data.Elem()
		case reflect.Interface:
			if data.NumMethod() == 0 {
				data = data.Elem()
				continue
			}
			fallthrough
		default:
			return nil, errors.New("only struct or map values are allowed")
		}
	}
}

type listIterator struct {
	docs [][]byte
}

func (l *listIterator) HasNext() bool {
	return len(l.docs) > 0
}

func (l *listIterator) NextTo(doc interface{}) error {
	if err := safeDecode(doc, l.docs[0]); err != nil {
		return err
	}
	l.docs = l.docs[1:]
	return nil
}

func (l *listIterator) maybeExtend(filters []Filter, v []byte) {
	var proto map[string]interface{}
	decode(&proto, v)

	if !matchesAllFilters(proto, filters) {
		return
	}

	copied := make([]byte, len(v))
	copy(copied, v)
	l.docs = append(l.docs, copied)
}

type filterAsserter struct {
	res *bool
}

func (f *filterAsserter) Errorf(_ string, _ ...interface{}) {
	*f.res = false
}

func doMatchFilter(value interface{}, f Filter) bool {
	matches := true
	t := filterAsserter{res: &matches}

	switch f.Op {
	case OpEqual:
		assert.EqualValues(&t, value, f.Value)
	case OpGreaterThan:
		assert.Greater(&t, value, f.Value)
	case OpLessThan:
		assert.Less(&t, value, f.Value)
	case OpGreaterThanEqual:
		assert.EqualValues(&t, value, f.Value)
		if !matches {
			matches = true
			assert.GreaterOrEqual(&t, value, f.Value)
		}
	case OpLessThanEqual:
		assert.EqualValues(&t, value, f.Value)
		if !matches {
			matches = true
			assert.LessOrEqual(&t, value, f.Value)
		}
	default:
		panic("unexpected op")
	}

	return matches
}

func matchFilter(proto map[string]interface{}, f Filter) bool {
	if len(f.FieldPath) == 1 {
		return doMatchFilter(proto[f.FieldPath[0]], f)
	}

	field, exist := proto[f.FieldPath[0]]
	if !exist {
		return false
	}

	m, ok := field.(map[string]interface{})
	if !ok {
		panic("corrupted record: field node is not a map")
	}

	f.FieldPath = f.FieldPath[1:]
	return matchFilter(m, f)
}

func matchesAllFilters(proto map[string]interface{}, filters []Filter) bool {
	for _, f := range filters {
		if !matchFilter(proto, f) {
			return false
		}
	}
	return true
}
