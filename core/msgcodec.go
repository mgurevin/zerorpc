package core

import (
	"io"
	"sync"
)

// MsgCodec ...
type MsgCodec interface {
	New() interface{}
	Encode(io.Writer, interface{}) error
	Decode(interface{}, io.Reader) error
}

type msgCodec struct {
	codec MsgCodec
	pool  *sync.Pool
}

func newCodec(c MsgCodec) *msgCodec {
	return &msgCodec{
		codec: c,
		pool: &sync.Pool{
			New: c.New,
		},
	}
}

func (c *msgCodec) getMsg() (interface{}, error) {
	return c.pool.Get(), nil
}

func (c *msgCodec) putMsg(msg interface{}) error {
	c.pool.Put(msg)

	return nil
}

func (c *msgCodec) Encode(dst io.Writer, src interface{}) error {
	return c.codec.Encode(dst, src)
}

func (c *msgCodec) Decode(dst interface{}, src io.Reader) error {
	return c.codec.Decode(dst, src)
}
