package encoding

// Marshaler is a text or binary marshaler that can be used
// by document.Service implementations to abstract document encoding.
type Marshaler interface {
	Marshal(in interface{}) ([]byte, error)
	Unmarshal(data []byte, to interface{}) error
}
