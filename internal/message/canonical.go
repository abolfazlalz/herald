package message

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
)

// Canonicalize produces deterministic JSON bytes for signing
func Canonicalize(input map[string]any) ([]byte, error) {
	if input == nil {
		return nil, errors.New("input is nil")
	}

	buf := &bytes.Buffer{}
	if err := encodeCanonical(buf, input); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeCanonical recursively sorts keys and encodes JSON
func encodeCanonical(buf *bytes.Buffer, data any) error {
	switch v := data.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		buf.WriteByte('{')
		for i, k := range keys {
			keyB, _ := json.Marshal(k)
			buf.Write(keyB)
			buf.WriteByte(':')

			if err := encodeCanonical(buf, v[k]); err != nil {
				return err
			}

			if i < len(keys)-1 {
				buf.WriteByte(',')
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, elem := range v {
			if err := encodeCanonical(buf, elem); err != nil {
				return err
			}
			if i < len(v)-1 {
				buf.WriteByte(',')
			}
		}
		buf.WriteByte(']')
	default:
		// string, number, bool, nil
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		buf.Write(b)
	}
	return nil
}
