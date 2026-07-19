/*
©AngelaMos | 2026
compress.go

Optional flate compression applied before encryption in the payload envelope
*/

package payload

import (
	"bytes"
	"compress/flate"
	"io"
)

func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(data))
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}
