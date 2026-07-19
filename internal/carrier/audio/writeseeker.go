/*
©AngelaMos | 2026
writeseeker.go

In-memory io.WriteSeeker so the WAV encoder can seek back and patch chunk sizes
*/

package audio

import (
	"errors"
	"io"
)

var (
	errNegativeSeek  = errors.New("crypha/audio: negative seek position")
	errInvalidWhence = errors.New("crypha/audio: invalid seek whence")
)

type memWriteSeeker struct {
	buf []byte
	pos int64
}

func (m *memWriteSeeker) Write(p []byte) (int, error) {
	end := m.pos + int64(len(p))
	if end > int64(len(m.buf)) {
		grown := make([]byte, end)
		copy(grown, m.buf)
		m.buf = grown
	}
	copy(m.buf[m.pos:end], p)
	m.pos = end
	return len(p), nil
}

func (m *memWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = m.pos + offset
	case io.SeekEnd:
		next = int64(len(m.buf)) + offset
	default:
		return 0, errInvalidWhence
	}
	if next < 0 {
		return 0, errNegativeSeek
	}
	m.pos = next
	return next, nil
}
