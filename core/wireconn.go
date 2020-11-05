package core

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

// ErrMalformedWireMsg ...
var ErrMalformedWireMsg = errors.New("malformed wire message")

// ErrRemote ...
type ErrRemote string

func (e ErrRemote) Error() string {
	return string(e)
}

type wireConn struct {
	r io.Reader
	w io.Writer

	flushW func() error
	closeR func() error
	closeW func() error

	errMu sync.RWMutex
	err   error

	readBuf  [8]byte
	writeBuf [8]byte
	errBuf   [255]byte

	payloadReader io.ReadCloser
	payloadWriter io.WriteCloser
}

func newWireConn(w io.Writer, r io.Reader) *wireConn {
	conn := &wireConn{
		w: w,
		r: r,
	}

	conn.payloadWriter = NewChunkedWriter(conn.w, MaxChunkSize)
	conn.payloadReader = NewChunkedReader(conn.r)

	if flusher, ok := w.(interface{ Flush() error }); ok {
		conn.flushW = flusher.Flush

	} else if flusher, ok := w.(interface{ Flush() }); ok {
		conn.flushW = func() error {
			flusher.Flush()

			return nil
		}
	}

	if closer, ok := w.(io.Closer); ok {
		conn.closeW = closer.Close
	}

	if closer, ok := r.(io.Closer); ok {
		conn.closeR = closer.Close
	}

	return conn
}

func (c *wireConn) writeMsg(msg *connMsg) (err error) {
	if err = c.hasErr(); err != nil {
		return err
	}

	defer c.setErr(err)

	// write operation code
	c.writeBuf[0] = byte(msg.op)
	if _, err = c.w.Write(c.writeBuf[:1]); err != nil {
		return err
	}

	//write call ID
	if msg.op != rpcOpPush {
		binary.BigEndian.PutUint64(c.writeBuf[:8], msg.callID)
		if _, err = c.w.Write(c.writeBuf[:8]); err != nil {
			return err
		}
	}

	// write method ID
	if msg.op == rpcOpCall || msg.op == rpcOpPush {
		binary.BigEndian.PutUint64(c.writeBuf[:8], msg.method)
		if _, err = c.w.Write(c.writeBuf[:8]); err != nil {
			return err
		}
	}

	switch msg.op {
	case rpcOpCall, rpcOpPush, rpcOpRet:
		// write message type
		binary.BigEndian.PutUint64(c.writeBuf[:8], msg.msgType)
		if _, err = c.w.Write(c.writeBuf[:8]); err != nil {
			return err
		}

		// write message payload
		if err = msg.msgCodec.Encode(c.payloadWriter, msg.msg); err != nil {
			return err

		} else if err = c.payloadWriter.Close(); err != nil {
			return err
		}

	case rpcOpErr:
		// write message type
		if e, ok := msg.msg.(error); !ok {
			return ErrMalformedWireMsg

		} else if eMsg := e.Error(); len(eMsg) == 0 || len(eMsg) > 255 {
			return ErrMalformedWireMsg

		} else {
			copy(c.errBuf[:], eMsg[:])

			c.writeBuf[0] = byte(len(eMsg))
			if _, err = c.w.Write(c.writeBuf[:1]); err != nil {
				return err

			} else if _, err = c.w.Write(c.errBuf[:len(eMsg)]); err != nil {
				return err
			}
		}

	case rpcOpErrUnimplemented:
		// do nothing

	default:
		panic("here is a bug")
	}

	// flush the buffer if exists on the underlying writer
	if c.flushW != nil {
		if err = c.flushW(); err != nil {
			return err
		}
	}

	return nil
}

func (c *wireConn) readMsg(msg *connMsg) (err error) {
	if err = c.hasErr(); err != nil {
		return err
	}

	// read operation code
	if _, err = io.ReadAtLeast(c.r, c.readBuf[:1], 1); err != nil {
		return c.setErr(err)

	} else if msg.op = rpcOp(c.readBuf[0]); !msg.op.valid() {
		return c.setErr(ErrMalformedWireMsg)
	}

	// read call ID
	if msg.op != rpcOpPush {
		if _, err = io.ReadAtLeast(c.r, c.readBuf[:8], 8); err != nil {
			return c.setErr(err)
		}
		msg.callID = binary.BigEndian.Uint64(c.readBuf[:8])
	}

	// read method ID
	if msg.op == rpcOpCall || msg.op == rpcOpPush {
		if _, err = io.ReadAtLeast(c.r, c.readBuf[:8], 8); err != nil {
			return c.setErr(err)
		}

		msg.method = binary.BigEndian.Uint64(c.readBuf[:8])
	}

	switch msg.op {
	case rpcOpCall, rpcOpPush, rpcOpRet:
		// read message type
		if _, err = io.ReadAtLeast(c.r, c.readBuf[:8], 8); err != nil {
			return c.setErr(err)
		}

		msg.msgType = binary.BigEndian.Uint64(c.readBuf[:8])

		// read message payload
		if msg.msgCodec, err = msg.p.findCodec(msg.msgType); err != nil {
			return c.setErr(err)

		} else if msg.msg, err = msg.msgCodec.getMsg(); err != nil {
			return c.setErr(err)

		} else if err = msg.msgCodec.Decode(msg.msg, c.payloadReader); err != nil {
			return c.setErr(err)

		} else if err = c.payloadReader.Close(); err != nil {
			return c.setErr(err)
		}

	case rpcOpErr:
		// read error message
		if _, err = io.ReadAtLeast(c.r, c.readBuf[:1], 1); err != nil {
			return c.setErr(err)

		} else if l := int(c.readBuf[0]); l == 0 {
			return c.setErr(ErrMalformedWireMsg)

		} else if _, err = io.ReadAtLeast(c.r, c.errBuf[:l], l); err != nil {
			return c.setErr(err)

		} else {
			errMsg := string(c.errBuf[:l])
			msg.msg = (*ErrRemote)(&errMsg)
		}

	case rpcOpErrUnimplemented:
		// do nothing

	default:
		panic("here is a bug")
	}

	return nil
}

func (c *wireConn) Close() (err error) {
	if c.closeW != nil {
		if e := c.closeW(); e != nil {
			err = e
		}
	}

	if c.closeR != nil {
		if e := c.closeR(); e != nil && err != nil {
			err = e
		}
	}

	return err
}

func (c *wireConn) setErr(err error) error {
	if err != nil {
		c.errMu.Lock()
		defer c.errMu.Unlock()

		c.err = err
	}

	return err
}

func (c *wireConn) hasErr() (err error) {
	c.errMu.RLock()
	defer c.errMu.RUnlock()

	return c.err
}
