package tts

import (
	"bytes"
	"errors"
)

var (
	ErrBufferFull = errors.New("buffer is already full")
)

// Buffer wraps bytes.Buffer and restricts its maximum size.
type Buffer struct {
	buffer  *bytes.Buffer
	maxSize int
}

// NewFixedSizeBuffer creates a new FixedSizeBuffer with the given max size.
func NewFixedSizeBuffer(maxSize int) *Buffer {
	b := make([]byte, 0, maxSize)
	return &Buffer{
		buffer:  bytes.NewBuffer(b),
		maxSize: maxSize,
	}
}

// Write appends data to the buffer.
// It returns error if the buffer exceeds its maximum size.
func (fb *Buffer) Write(p []byte) (int, error) {
	available := fb.buffer.Available()
	if available == 0 {
		return 0, ErrBufferFull
	}

	if len(p) > available {
		p = p[:available]
	}

	n, err := fb.buffer.Write(p)
	if err != nil {
		return n, err
	}

	if fb.buffer.Len() == fb.maxSize {
		return n, ErrBufferFull
	}

	return n, nil
}

// Reset resets the buffer
func (fb *Buffer) Reset() {
	fb.buffer.Reset()
}

// String returns the contents of the buffer as a string.
func (fb *Buffer) String() string {
	return fb.buffer.String()
}
