package core

import (
	"encoding/binary"
	"io"
	"math"
)

// MaxChunkSize ...
const MaxChunkSize = 1<<16 - 1

var chunkedByteOrder = binary.BigEndian

type chunkedWriter struct {
	io.Writer

	size      int
	buf       []byte
	shortBuf  [2]byte
	bufOffset int
}

type chunkedReader struct {
	io.Reader

	shortBuf  [2]byte
	remaining int
}

// ChunkedSize ...
func ChunkedSize(n int64, chunkSize int64) int64 {
	if n == 0 {
		return 2
	}

	return n + int64(math.Ceil(float64(n)/float64(chunkSize)))*2 + 2
}

// NewChunkedWriter ...
func NewChunkedWriter(w io.Writer, chunkSize int) io.WriteCloser {
	if chunkSize > MaxChunkSize {
		panic("chunk size must be fit in two bytes")
	}

	return &chunkedWriter{
		Writer: w,
		buf:    make([]byte, chunkSize),
		size:   chunkSize,
	}
}

// NewChunkedReader ...
func NewChunkedReader(r io.Reader) io.ReadCloser {
	return &chunkedReader{
		Reader: r,
	}
}

func (w *chunkedWriter) Write(p []byte) (n int, err error) {
	for lp := len(p); n != lp; {
		c := copy(w.buf[w.bufOffset:], p[n:])

		n += c
		w.bufOffset += c

		if w.bufOffset == w.size {
			if err = w.flush(); err != nil {
				return n, err
			}
		}
	}

	return n, nil
}

func (w *chunkedWriter) flush() (err error) {
	if w.bufOffset == 0 {
		return nil
	}

	chunkedByteOrder.PutUint16(w.shortBuf[:2], uint16(w.bufOffset))
	if _, err = w.Writer.Write(w.shortBuf[:2]); err != nil {
		return err
	}

	if _, err = w.Writer.Write(w.buf[:w.bufOffset]); err != nil {
		return err
	}

	w.bufOffset = 0

	return nil
}

func (w *chunkedWriter) Close() (err error) {
	if err = w.flush(); err != nil {
		return err
	}

	chunkedByteOrder.PutUint16(w.shortBuf[:2], uint16(0))
	if _, err = w.Writer.Write(w.shortBuf[:2]); err != nil {
		return err
	}

	return nil
}

func (r *chunkedReader) Read(p []byte) (n int, err error) {
	if r.remaining == -1 {
		return 0, io.EOF
	}

	pOff := 0
	pMax := len(p)

	for pLen := pMax - pOff; pLen > 0; pLen = pMax - pOff {
		if r.remaining == 0 {
			if _, err = io.ReadAtLeast(r.Reader, r.shortBuf[:2], 2); err != nil {
				return n, err
			}

			r.remaining = int(chunkedByteOrder.Uint16(r.shortBuf[:2]))
			if r.remaining == 0 {
				r.remaining = -1
				return n, io.EOF
			}
		}

		read := r.remaining
		if pLen < read {
			read = pLen
		}

		pn, err := io.ReadAtLeast(r.Reader, p[pOff:pOff+read], read)
		n += pn

		if err != nil {
			return n, err
		}

		r.remaining -= pn
		pOff += pn

		if r.remaining == 0 {
			if _, err = io.ReadAtLeast(r.Reader, r.shortBuf[:2], 2); err != nil {
				return n, err
			}

			r.remaining = int(chunkedByteOrder.Uint16(r.shortBuf[:2]))
			if r.remaining == 0 {
				r.remaining = -1
				return n, nil
			}
		}
	}

	return n, nil
}

func (r *chunkedReader) Close() error {
	r.remaining = 0

	return nil
}
