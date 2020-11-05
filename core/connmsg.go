package core

import (
	"context"
	"errors"
	"sync/atomic"
)

type rpcOp byte

const (
	rpcOpCall             rpcOp = 0x01
	rpcOpPush             rpcOp = 0x02
	rpcOpRet              rpcOp = 0x03
	rpcOpErr              rpcOp = 0x04
	rpcOpErrUnimplemented rpcOp = 0x05
)

// ErrResponseAlreadyPrepared ...
var ErrResponseAlreadyPrepared = errors.New("response already prepared")

// ErrResponseAlreadySentOrNotPrepared ...
var ErrResponseAlreadySentOrNotPrepared = errors.New("response already sent or not prepared")

// ErrUnimplementedRPC ...
var ErrUnimplementedRPC = errors.New("unimplemented rpc")

const (
	respStateNew      = int64(0)
	respStatePrepared = int64(1)
	respStateSent     = int64(2)
)

type connMsg struct {
	p       *rpcConn
	errCh   chan error
	replyCh chan *connMsg

	ctx          context.Context
	respState    int64
	callReleased int64

	op       rpcOp
	callID   uint64
	method   uint64
	msgType  uint64
	msg      interface{}
	msgCodec *msgCodec
}

func (m *connMsg) PrepareResponse(messageTypeID uint64) (msg interface{}, err error) {
	if atomic.CompareAndSwapInt64(&m.callReleased, 0, 1) {
		if err := m.msgCodec.putMsg(m.msg); err != nil {
			return nil, err
		}
	}

	if atomic.CompareAndSwapInt64(&m.respState, respStateNew, respStatePrepared) {
		m.msgType = messageTypeID

		m.msgCodec, err = m.p.findCodec(messageTypeID)
		if err != nil {
			return nil, err
		}

		m.msg, err = m.msgCodec.getMsg()
		if err != nil {
			return nil, err
		}

		return m.msg, nil
	}

	return ErrResponseAlreadyPrepared, nil
}

func (m *connMsg) SendResponse() error {
	if atomic.CompareAndSwapInt64(&m.respState, respStatePrepared, respStateSent) {
		defer m.msgCodec.putMsg(m.msg)

		m.op = rpcOpRet

		select {
		case <-m.ctx.Done():
			return m.ctx.Err()

		case m.p.sendCh <- m:
			select {
			case <-m.ctx.Done():
				return m.ctx.Err()

			case err := <-m.errCh:
				return err
			}
		}
	}

	return ErrResponseAlreadySentOrNotPrepared
}

func (m *connMsg) SendError(e error) error {
	if atomic.CompareAndSwapInt64(&m.respState, respStatePrepared, respStateSent) {
		return m.sendError(e)
	}

	return ErrResponseAlreadySentOrNotPrepared
}

func (m *connMsg) sendError(e error) error {
	m.msgCodec.putMsg(m.msg)

	if errors.Is(e, ErrUnimplementedRPC) {
		m.op = rpcOpErrUnimplemented
		m.msg = nil
		m.msgCodec = nil

	} else {
		m.op = rpcOpErr
		m.msg = e
		m.msgCodec = nil
	}

	select {
	case <-m.ctx.Done():
		return m.ctx.Err()

	case m.p.sendCh <- m:
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()

		case err := <-m.errCh:
			return err
		}
	}
}

func (m *connMsg) reset() *connMsg {
	m.ctx = nil
	m.respState = respStateNew
	m.callReleased = 0

	m.op = 0
	m.callID = 0
	m.method = 0
	m.msgType = 0
	m.msg = nil
	m.msgCodec = nil

	return m
}

func (m *connMsg) Context() context.Context {
	return m.ctx
}

func (m *connMsg) MethodID() uint64 {
	return m.method
}

func (m *connMsg) MessageTypeID() uint64 {
	return m.msgType
}

func (m *connMsg) Message() interface{} {
	return m.msg
}

func (o rpcOp) valid() bool {
	switch o {
	case rpcOpCall, rpcOpPush, rpcOpRet, rpcOpErr, rpcOpErrUnimplemented:
		return true

	default:
		return false
	}
}
