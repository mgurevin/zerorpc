package core

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkedSize(t *testing.T) {
	assert.Equal(t, int64(0+2), ChunkedSize(0, MaxChunkSize))
	assert.Equal(t, int64(2+1+2), ChunkedSize(1, MaxChunkSize))
	assert.Equal(t, int64(2+MaxChunkSize+2), ChunkedSize(MaxChunkSize, MaxChunkSize))
	assert.Equal(t, int64(2+MaxChunkSize+2+1+2), ChunkedSize(MaxChunkSize+1, MaxChunkSize))
}

func TestChunkedEmptyRW(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	w := NewChunkedWriter(buf, MaxChunkSize)

	// attempt to write zero bytes
	n, err := w.Write(make([]byte, 0))

	assert.Nil(t, err)    // should be no error
	assert.Equal(t, 0, n) // should be zero

	assert.Equal(t, 0, buf.Len()) // no bytes should be written to the wire at this point
	assert.Nil(t, w.Close())      // closing also should be no error

	b := buf.Bytes()

	assert.Len(t, b, 2)                                     // should be written to the wire just EOF signature
	assert.Equal(t, 0, int(chunkedByteOrder.Uint16(b[:2]))) // so it's zero in the form of short

	r := NewChunkedReader(bytes.NewBuffer(b))

	// attempt to read zero bytes
	rb := make([]byte, 0)
	n, err = r.Read(rb)

	assert.Nil(t, err)    // should be no error
	assert.Equal(t, 0, n) // should be read zero bytes

	assert.Nil(t, r.Close()) // closing should not be errored

	// attempt to read zero bytes from closed reader
	rb = make([]byte, 0)
	n, err = r.Read(rb)

	assert.Nil(t, err)    // should be no error
	assert.Equal(t, 0, n) // should be read zero bytes

	// attempt to read more than zero bytes from closed reader
	rb = []byte{0x42}
	n, err = r.Read(rb)

	assert.Equal(t, io.EOF, err)            // should be give us that EOF error
	assert.Equal(t, 0, n)                   // should be zero
	assert.EqualValues(t, []byte{0x42}, rb) // should be don't touch buffer

	r = NewChunkedReader(bytes.NewBuffer(b))

	// attempt to read more than zero bytes from empty reader
	rb = []byte{0x42}
	n, err = r.Read(rb)

	assert.Equal(t, io.EOF, err)            // should be give us that EOF error
	assert.Equal(t, 0, n)                   // should be zero
	assert.EqualValues(t, []byte{0x42}, rb) // should be don't touch buffer
}

func TestChunkedShortRW(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	w := NewChunkedWriter(buf, 5)

	n, err := w.Write([]byte("test"))
	assert.Nil(t, err)
	assert.Equal(t, 4, n)

	assert.Nil(t, w.Close())

	b := buf.Bytes()

	assert.Len(t, b, 2+4+2)
	assert.Equal(t, 4, int(chunkedByteOrder.Uint16(b[:2])))
	assert.EqualValues(t, []byte("test"), b[2:6])
	assert.Equal(t, 0, int(chunkedByteOrder.Uint16(b[6:8])))

	r := NewChunkedReader(bytes.NewBuffer(b))

	rb := make([]byte, 0)
	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 0, n)

	rb = make([]byte, 1)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("t"), rb)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("e"), rb)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("s"), rb)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("t"), rb)

	n, err = r.Read(rb)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

func TestChunkedFullRW(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	w := NewChunkedWriter(buf, 16)

	n, err := w.Write([]byte("testtesttesttest"))
	assert.Nil(t, err)
	assert.Equal(t, 16, n)

	assert.Nil(t, w.Close())

	b := buf.Bytes()

	assert.Len(t, b, 2+16+2)
	assert.Equal(t, 16, int(chunkedByteOrder.Uint16(b[:2])))
	assert.EqualValues(t, []byte("testtesttesttest"), b[2:18])
	assert.Equal(t, 0, int(chunkedByteOrder.Uint16(b[18:20])))

	r := NewChunkedReader(bytes.NewBuffer(b))

	rb := make([]byte, 16)
	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 16, n)
	assert.EqualValues(t, []byte("testtesttesttest"), rb)

	rb = make([]byte, 1)
	n, err = r.Read(rb)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

func TestChunkedFull2Write(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	w := NewChunkedWriter(buf, 2)

	n, err := w.Write([]byte("test"))
	assert.Nil(t, err)
	assert.Equal(t, 4, n)

	assert.Nil(t, w.Close())

	b := buf.Bytes()

	assert.Len(t, b, 2+2+2+2+2)
	assert.Equal(t, 2, int(chunkedByteOrder.Uint16(b[:2])))
	assert.EqualValues(t, []byte("te"), b[2:4])
	assert.Equal(t, 2, int(chunkedByteOrder.Uint16(b[4:6])))
	assert.EqualValues(t, []byte("st"), b[6:8])
	assert.Equal(t, 0, int(chunkedByteOrder.Uint16(b[8:10])))

	r := NewChunkedReader(bytes.NewBuffer(b))

	rb := make([]byte, 5)
	rb[4] = 'X'
	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 4, n)
	assert.EqualValues(t, []byte("testX"), rb)

	rb = make([]byte, 1)
	n, err = r.Read(rb)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	r = NewChunkedReader(bytes.NewBuffer(b))

	rb = make([]byte, 0)
	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 0, n)

	rb = make([]byte, 1)
	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("t"), rb)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("e"), rb)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("s"), rb)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("t"), rb)

	n, err = r.Read(rb)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

func TestChunkedOverWrite(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	w := NewChunkedWriter(buf, 4)

	n, err := w.Write([]byte("testt"))
	assert.Nil(t, err)
	assert.Equal(t, 5, n)

	assert.Nil(t, w.Close())

	b := buf.Bytes()

	assert.Len(t, b, 2+4+2+1+2)
	assert.Equal(t, 4, int(chunkedByteOrder.Uint16(b[:2])))
	assert.EqualValues(t, []byte("test"), b[2:6])
	assert.Equal(t, 1, int(chunkedByteOrder.Uint16(b[6:8])))
	assert.EqualValues(t, []byte("t"), b[8:9])
	assert.Equal(t, 0, int(chunkedByteOrder.Uint16(b[9:11])))

	r := NewChunkedReader(bytes.NewBuffer(b))

	rb := make([]byte, 5)
	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	assert.EqualValues(t, []byte("testt"), rb)
}
func TestChunkedOver2Write(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	w := NewChunkedWriter(buf, 2)

	n, err := w.Write([]byte("testt"))
	assert.Nil(t, err)
	assert.Equal(t, 5, n)

	assert.Nil(t, w.Close())

	b := buf.Bytes()

	assert.Len(t, b, 2+2+2+2+2+1+2)
	assert.Equal(t, 2, int(chunkedByteOrder.Uint16(b[:2])))
	assert.EqualValues(t, []byte("te"), b[2:4])
	assert.Equal(t, 2, int(chunkedByteOrder.Uint16(b[4:6])))
	assert.EqualValues(t, []byte("st"), b[6:8])
	assert.Equal(t, 1, int(chunkedByteOrder.Uint16(b[8:10])))
	assert.EqualValues(t, []byte("t"), b[10:11])
	assert.Equal(t, 0, int(chunkedByteOrder.Uint16(b[11:13])))

	r := NewChunkedReader(bytes.NewBuffer(b))

	rb := make([]byte, 4)
	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 4, n)
	assert.EqualValues(t, []byte("test"), rb)

	n, err = r.Read(rb)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, []byte("test"), rb)
}

func BenchmarkWrite(b *testing.B) {
	r := &io.LimitedReader{
		R: &zeroReader{},
	}

	t := func(b *testing.B, w io.WriteCloser, n int64, buf []byte, rewind func()) {
		for i := 0; i < b.N; i++ {
			r.N = n
			wn, err := io.CopyBuffer(w, r, buf)
			if err != nil {
				b.Fatal(err)
			}

			if wn != n {
				b.Fatalf("write error, n: %d, wn: %d", n, wn)
			}

			if err = w.Close(); err != nil {
				b.Fatal(err)
			}

			b.SetBytes(wn)

			rewind()
		}
	}

	for i := 0; i < 24; i++ {
		n := int64(1 << i)

		bw := &bufWriter{buf: make([]byte, ChunkedSize(n, MaxChunkSize))}
		rewind := func() {
			bw.n = 0
		}

		buf := make([]byte, 4*1024)

		w := NewChunkedWriter(bw, MaxChunkSize)
		b.Run(byteCountIEC(n)+"-Chunked", func(b *testing.B) {
			b.ReportAllocs()
			t(b, w, n, buf, rewind)
		})

		b.Run(byteCountIEC(n)+"-RAW    ", func(b *testing.B) {
			b.ReportAllocs()
			t(b, bw, n, buf, rewind)
		})
	}
}

func BenchmarkRead(b *testing.B) {
	bufs := make([]*bufWriter, 24)

	for i := 0; i < 24; i++ {
		n := int64(1 << i)

		chunkedSize := ChunkedSize(n, MaxChunkSize)

		bufs[i] = &bufWriter{buf: make([]byte, chunkedSize)}

		w := NewChunkedWriter(bufs[i], MaxChunkSize)
		r := &io.LimitedReader{R: &zeroReader{}, N: n}

		wn, err := io.Copy(w, r)
		if err != nil {
			b.Fatal(err)
		}

		if wn != n {
			b.Fatalf("write error, n: %d, wn: %d", n, wn)
		}

		if err := w.Close(); err != nil {
			b.Fatal(err)
		}

		if int64(bufs[i].n) != chunkedSize {
			b.Fatalf("ChunkedSize error, expected: %d, got: %d\n", chunkedSize, bufs[i].n)
		}
	}

	t := func(b *testing.B, r io.ReadCloser, w io.Writer, n int64, rewind func(), buf []byte) {
		for i := 0; i < b.N; i++ {
			rn, err := io.CopyBuffer(w, r, buf)
			if err != nil {
				b.Fatal(err)
			}

			if rn != n {
				b.Fatalf("read error, n: %d, rn: %d", n, rn)
			}

			if err := r.Close(); err != nil {
				b.Fatal(err)
			}

			rewind()

			b.SetBytes(rn)
		}
	}

	for i := 0; i < 24; i++ {
		n := int64(1 << i)

		buf := make([]byte, n*2)

		r := bytes.NewReader(bufs[i].buf)

		rewind := func() {
			r.Seek(0, io.SeekStart)
		}

		b.Run(byteCountIEC(n)+"-Chunked", func(b *testing.B) {
			b.ReportAllocs()
			t(b, NewChunkedReader(r), ioutil.Discard, n, rewind, buf)
		})

		lr := &io.LimitedReader{R: r, N: n}

		rawRewind := func() {
			r.Seek(0, io.SeekStart)
			lr.N = n
		}

		b.Run(byteCountIEC(n)+"-RAW    ", func(b *testing.B) {
			b.ReportAllocs()
			t(b, ioutil.NopCloser(lr), ioutil.Discard, n, rawRewind, buf)
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (n int, err error) {
	return len(p), nil
}

type bufWriter struct {
	n   int64
	buf []byte
}

func (w *bufWriter) Write(p []byte) (n int, err error) {
	n = copy(w.buf[w.n:], p)
	w.n += int64(n)
	return n, nil
}

func (w *bufWriter) Close() error {
	w.n = 0
	return nil
}

func byteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
