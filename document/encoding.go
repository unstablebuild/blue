package document

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

// UpdateUpdatedAtField updates the default UpdatedAt field in the given document.
func UpdateUpdatedAtField(doc interface{}) interface{} {
	now := time.Now()
	m, ok := doc.(map[string]interface{})
	if ok {
		doc = setField(m, DefaultUpdatedAtField, now)
	} else {
		dst := clone(doc)
		reflectSetTimeField(dst, DefaultUpdatedAtField, now)
		doc = dst.Elem().Interface()
	}
	return doc
}

// UpdateCreatedAtField updates the default CreatedAt field in the given document
// and the default UpdatedAt field.
func UpdateCreatedAtField(doc interface{}) interface{} {
	m, ok := doc.(map[string]interface{})
	if ok {
		doc = setMapUpdatedAtFields(m)
	} else {
		doc = setStructUpdatedAtFields(doc)
	}
	return doc
}

// Encode encodes doc into a reversible format (via Decode)
// and returns the data in bytes.
func Encode(doc interface{}, addCreatedAt bool) []byte {
	if addCreatedAt {
		doc = UpdateCreatedAtField(doc)
	} else {
		doc = UpdateUpdatedAtField(doc)
	}

	b, err := bson.Marshal(doc)
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

	if !matchesAllFiltersBson(proto, filters) {
		return
	}

	copied := make([]byte, len(v))
	copy(copied, v)
	l.docs = append(l.docs, copied)
}

// UpdateProto updates proto with the given slice of updates.
func UpdateProto(lowerCase bool, updates []Update,
	proto map[string]interface{}, preconds ...Precondition) error {
	for _, cond := range preconds {
		if len(cond.FieldPath) == 0 {
			panic("empty field path")
		}

		fieldPath := cond.FieldPath
		if lowerCase {
			fieldPath = make([]string, len(cond.FieldPath))
			for i, comp := range cond.FieldPath {
				fieldPath[i] = strings.ToLower(comp)
			}
		}

		if !precondField(proto, Precondition{FieldPath: fieldPath, Value: cond.Value}) {
			return ErrPreconditionFailed
		}
	}

	for _, update := range updates {
		if len(update.FieldPath) == 0 {
			panic("empty field path")
		}
		if update.FieldPath[0] == DefaultUpdatedAtField {
			continue
		}

		// bson decodes struct fields into a map as lower case
		fieldPath := update.FieldPath
		if lowerCase {
			fieldPath = make([]string, len(update.FieldPath))
			for i, comp := range update.FieldPath {
				fieldPath[i] = strings.ToLower(comp)
			}
		}

		updateField(proto, Update{FieldPath: fieldPath, Value: update.Value})
	}

	updatedAtField := DefaultUpdatedAtField
	if lowerCase {
		updatedAtField = strings.ToLower(updatedAtField)
	}
	updateField(proto, Update{
		FieldPath: []string{updatedAtField},
		Value:     time.Now(),
	})

	return nil
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

func setField(m map[string]interface{}, key string, value interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, v := range m {
		ret[k] = v
	}
	ret[key] = value
	return ret
}

func setMapUpdatedAtFields(m map[string]interface{}) map[string]interface{} {
	now := time.Now()
	m = setField(m, DefaultUpdatedAtField, now)
	m = setField(m, DefaultCreatedAtField, now)
	return m
}

func setStructUpdatedAtFields(data interface{}) interface{} {
	dst := clone(data)
	now := time.Now()
	reflectSetTimeField(dst, DefaultCreatedAtField, now)
	reflectSetTimeField(dst, DefaultUpdatedAtField, now)
	return dst.Elem().Interface()
}

func precondField(proto map[string]interface{}, cond Precondition) bool {
	if len(cond.FieldPath) == 1 {
		return proto[cond.FieldPath[0]] == cond.Value
	}

	field, exist := proto[cond.FieldPath[0]]
	if !exist {
		return false
	}

	m, ok := field.(map[string]interface{})
	if !ok {
		panic("corrupted record: field node is not a map")
	}

	cond.FieldPath = cond.FieldPath[1:]
	return precondField(m, cond)
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

// MatchFilter returns true if proto satisfies the filter condition of f.
func MatchFilter(proto map[string]interface{}, f Filter) bool {
	if len(f.FieldPath) == 1 {
		return doMatchFilter(proto[f.FieldPath[0]], f)
	}

	field, exist := proto[f.FieldPath[0]]
	if !exist {
		return false
	}

	m := field.(map[string]interface{})
	f.FieldPath = f.FieldPath[1:]
	return MatchFilter(m, f)
}

func matchesAllFiltersBson(proto map[string]interface{}, filters []Filter) bool {
	for _, f := range filters {
		// bson decodes struct fields into a map as lower case
		lower := make([]string, len(f.FieldPath))
		for i, comp := range f.FieldPath {
			lower[i] = strings.ToLower(comp)
		}
		f.FieldPath = lower

		if !MatchFilter(proto, f) {
			return false
		}
	}
	return true
}
