package document

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func reflectSetTimeField(s reflect.Value, k string, v time.Time) {
	f := s.Elem().FieldByName(k)
	if !f.IsValid() || !f.CanSet() || f.Kind() != reflect.Struct ||
		reflect.TypeOf(f) == reflect.TypeOf((*time.Time)(nil)).Elem() {
		return
	}
	f.Set(reflect.ValueOf(v))
}

func clone(data interface{}) reflect.Value {
	typ := reflect.TypeOf(data)
	src := reflect.ValueOf(data)
	dst := reflect.New(typ)
	for i := 0; i < src.NumField(); i++ {
		dstField := dst.Elem().Field(i)
		if dstField.CanSet() {
			dstField.Set(src.Field(i))
		}
	}
	return dst
}

func setMapUpdatedAtFields(m map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, v := range m {
		ret[k] = v
	}
	now := time.Now()
	ret[DefaultCreatedAtField] = now
	ret[DefaultUpdatedAtField] = now
	return ret
}

func setStructUpdatedAtFields(data interface{}) interface{} {
	dst := clone(data)
	now := time.Now()
	reflectSetTimeField(dst, DefaultCreatedAtField, now)
	reflectSetTimeField(dst, DefaultUpdatedAtField, now)
	return dst.Elem().Interface()
}

func encode(data interface{}, addCreatedAt bool) []byte {
	if addCreatedAt {
		m, ok := data.(map[string]interface{})
		if ok {
			data = setMapUpdatedAtFields(m)
		} else {
			data = setStructUpdatedAtFields(data)
		}
	}

	b, err := bson.Marshal(data)
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
	err := bson.Unmarshal(raw, rcv)
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

		// bson decodes struct fields into a map as lower case
		lower := make([]string, len(update.FieldPath))
		for i, comp := range update.FieldPath {
			lower[i] = strings.ToLower(comp)
		}

		updateField(proto, Update{FieldPath: lower, Value: update.Value})
	}

	updateField(proto, Update{
		FieldPath: []string{strings.ToLower(DefaultUpdatedAtField)},
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

func (l *listIterator) Close() error {
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
		// bson decodes struct fields into a map as lower case
		lower := make([]string, len(f.FieldPath))
		for i, comp := range f.FieldPath {
			lower[i] = strings.ToLower(comp)
		}
		f.FieldPath = lower

		if !matchFilter(proto, f) {
			return false
		}
	}
	return true
}

// ListIterator returns an iterator that iterates over docs.
func ListIterator(docs ...interface{}) Iterator {
	iter := &listIterator{docs: make([][]byte, 0)}
	for _, data := range docs {
		iter.maybeExtend(nil, encode(data, false))
	}
	return iter
}
