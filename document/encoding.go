package document

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

// Encode encodes data into a reversible format (via Decode)
// and returns the data in bytes.
func Encode(data interface{}, addCreatedAt bool) []byte {
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

// SafeDecode checks if the given interface would be decoded by Decode
// and decodes it or otherwise returns an error.
func SafeDecode(rcv interface{}, raw []byte) error {
	if !IsEncodeable(rcv) {
		return errors.New("receiver is not a pointer and not a map or is nil")
	}
	Decode(rcv, raw)
	return nil
}

// IsEncodeable returns true if doc is a structure that can be safely
// decoded via Decode.
func IsEncodeable(doc interface{}) bool {
	v := reflect.ValueOf(doc)
	return (v.Kind() == reflect.Ptr || v.Kind() == reflect.Map) && !v.IsNil()
}

// Decode decotes raw into rcv and panics if there's an error decoding.
// Use SafeDecode if you are not sure if the structure rcv is safe to be
// encoded/decoded.
func Decode(rcv interface{}, raw []byte) {
	err := bson.Unmarshal(raw, rcv)
	if err != nil {
		panic(err)
	}
}

// ListIterator returns an iterator that iterates over docs.
func NewListIterator(docs ...interface{}) *ListIterator {
	iter := &ListIterator{docs: make([][]byte, 0)}
	for _, data := range docs {
		iter.Extend(nil, Encode(data, false))
	}
	return iter
}

// ListIterator satisfies an Iterator with an inmemory
// list of documents.
type ListIterator struct {
	docs [][]byte
}

// HasNext returns false if this Iterator is empty.
func (l *ListIterator) HasNext() bool {
	return len(l.docs) > 0
}

// NextTo decodes the next chunk of data into doc or returns
// an error if there was a decoding issue.
func (l *ListIterator) NextTo(doc interface{}) error {
	if err := SafeDecode(doc, l.docs[0]); err != nil {
		return err
	}
	l.docs = l.docs[1:]
	return nil
}

// Close does nothing.
func (l *ListIterator) Close() error {
	return nil
}

// Extend extends this iterator if and only if the data chunk's
// structure satisfies all filters.
func (l *ListIterator) Extend(filters []Filter, v []byte) {
	var proto map[string]interface{}
	Decode(&proto, v)

	if !matchesAllFilters(proto, filters) {
		return
	}

	copied := make([]byte, len(v))
	copy(copied, v)
	l.docs = append(l.docs, copied)
}

// UpdateProto updates proto with the given slice of updates.
func UpdateProto(updates []Update, proto map[string]interface{}) {
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

// DerefCreateValue dereferences data for a service.Create implementation
// until it finds a structure that can be used for Encode/Decode
// or returns an error if no such structure could be found.
func DerefCreateValue(data reflect.Value) (interface{}, error) {
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
