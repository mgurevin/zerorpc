package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// ErrCodecNotFound ...
var ErrCodecNotFound = errors.New("codec not found")

// MessageFunc ...
type MessageFunc func(message interface{}) error

// Call ...
type Call interface {
	Context() context.Context

	MethodID() uint64
	MessageTypeID() uint64
	Message() interface{}

	PrepareResponse(messageTypeID uint64) (interface{}, error)
	SendResponse() error
	SendError(err error) error
}

type rpcConn struct {
	ctx context.Context

	msgPool  *sync.Pool
	newCnt   uint64
	poolSize int64

	seq uint64

	sendCh chan *connMsg
	recvCh chan *connMsg

	waitMu sync.RWMutex
	wait   map[uint64]chan *connMsg

	mtMu   sync.RWMutex
	codecs map[uint64]*msgCodec

	handler func(Call) error
}

func newRPCConn(ctx context.Context, handler func(Call) error) *rpcConn {
	p := &rpcConn{
		ctx: ctx,

		sendCh:  make(chan *connMsg),
		recvCh:  make(chan *connMsg),
		wait:    make(map[uint64]chan *connMsg),
		codecs:  make(map[uint64]*msgCodec),
		handler: handler,
	}

	p.msgPool = &sync.Pool{
		New: func() interface{} {
			atomic.AddUint64(&p.newCnt, 1)

			return &connMsg{
				p:       p,
				errCh:   make(chan error),
				replyCh: make(chan *connMsg),
			}
		},
	}

	return p
}

func (p *rpcConn) getConnMsg() *connMsg {
	atomic.AddInt64(&p.poolSize, 1)

	return p.msgPool.Get().(*connMsg)
}

func (p *rpcConn) putConnMsg(msg *connMsg) {
	p.msgPool.Put(msg.reset())

	atomic.AddInt64(&p.poolSize, ^0)
}

func (p *rpcConn) RegisterMessageType(typeID uint64, codec MsgCodec) {
	p.mtMu.Lock()
	defer p.mtMu.Unlock()

	p.codecs[typeID] = newCodec(codec)
}

func (p *rpcConn) findCodec(typeID uint64) (*msgCodec, error) {
	p.mtMu.RLock()
	defer p.mtMu.RUnlock()

	codec, ok := p.codecs[typeID]
	if !ok {
		return nil, ErrCodecNotFound
	}

	return codec, nil
}

func (p *rpcConn) NewCodecMsg(typeID uint64) (interface{}, error) {
	p.mtMu.RLock()
	defer p.mtMu.RUnlock()

	codec, ok := p.codecs[typeID]
	if !ok {
		return nil, ErrCodecNotFound
	}

	return codec.getMsg()
}

func (p *rpcConn) ReleaseCodecMsg(typeID uint64, msg interface{}) error {
	p.mtMu.RLock()
	defer p.mtMu.RUnlock()

	codec, ok := p.codecs[typeID]
	if !ok {
		return ErrCodecNotFound
	}

	return codec.putMsg(msg)
}

func (p *rpcConn) handleCallMsg(msg *connMsg) {
	defer p.putConnMsg(msg)
	defer msg.msgCodec.putMsg(msg.msg)

	msg.ctx = p.ctx

	if p.handler == nil {
		if err := msg.SendError(ErrUnimplementedRPC); err != nil {
			// log
			return
		}
	}

	hErr := p.handler(msg)
	if hErr != nil {
		// log
		_ = hErr
	}

	if atomic.CompareAndSwapInt64(&msg.respState, respStateNew, respStateSent) {
		if err := msg.sendError(ErrUnimplementedRPC); err != nil {
			// log
			return
		}

	} else if atomic.CompareAndSwapInt64(&msg.respState, respStatePrepared, respStateSent) {
		msg.msgCodec.putMsg(msg.msg)

		if err := msg.sendError(ErrUnimplementedRPC); err != nil {
			// log
			return
		}

	} else if atomic.CompareAndSwapInt64(&msg.respState, respStateSent, respStateSent) {
		msg.msgCodec.putMsg(msg.msg)
	}

	return
}

func (p *rpcConn) handlePushMsg(msg *connMsg) {
	defer p.putConnMsg(msg)
	defer msg.msgCodec.putMsg(msg.msg)

	if p.handler == nil {
		// log
		return
	}

	msg.ctx = p.ctx

	atomic.StoreInt64(&msg.respState, respStateSent)

	hErr := p.handler(msg)
	if hErr != nil {
		// log
		_ = hErr
	}

	return
}

func (p *rpcConn) handleIncomingMsg() {
	for {
		select {
		case <-p.ctx.Done():
			//log
			return

		case msg, ok := <-p.recvCh:
			if !ok {
				// log
				return
			}

			switch msg.op {
			case rpcOpCall:
				go p.handleCallMsg(msg)

			case rpcOpPush:
				go p.handlePushMsg(msg)

			case rpcOpRet, rpcOpErr, rpcOpErrUnimplemented:
				p.waitMu.RLock()
				replyCh, ok := p.wait[msg.callID]
				p.waitMu.RUnlock()

				if !ok {
					// log unexpected msg
					fmt.Printf("%p unexpected msg:callID: %d - %d - op %d\n", p, msg.callID, msg.msgType, msg.op)
					p.putConnMsg(msg)
					continue
				}

				select {
				case <-p.ctx.Done():
					p.putConnMsg(msg)

				case replyCh <- msg:
				}

			default:
				panic("here is a bug")
			}
		}
	}
}

func (p *rpcConn) handleConn(ctx context.Context, w io.Writer, r io.Reader) error {
	connCtx, connCancel := context.WithCancel(p.ctx)

	go func() {
		select {
		case <-ctx.Done():
			connCancel()

		case <-connCtx.Done():
			return
		}
	}()

	conn := newWireConn(w, r)
	defer conn.Close()

	recvErr := make(chan error)
	sendErr := make(chan error)

	go func() {
		defer connCancel()
		defer close(recvErr)

		recvErr <- p.recvLoop(connCtx, conn)
	}()

	go func() {
		defer connCancel()
		defer close(sendErr)

		sendErr <- p.sendLoop(connCtx, conn)
	}()

	select {
	case <-connCtx.Done():
		return connCtx.Err()

	case err := <-recvErr:
		return err

	case err := <-sendErr:
		return err
	}
}

func (p *rpcConn) recvLoop(connCtx context.Context, wire *wireConn) error {
	for {
		select {
		case <-connCtx.Done():
			return connCtx.Err()

		default:
			msg := p.getConnMsg()

			if err := func() (err error) {
				defer func() {
					if e := recover(); e != nil {
						err = fmt.Errorf("%v", e)
					}

					if err != nil {
						p.putConnMsg(msg)
					}
				}()

				return wire.readMsg(msg)

			}(); err != nil {
				return err
			}

			select {
			case <-connCtx.Done():
				p.putConnMsg(msg)

				return connCtx.Err()

			case p.recvCh <- msg:
			}
		}
	}
}

func (p *rpcConn) sendLoop(ctx context.Context, wire *wireConn) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok := <-p.sendCh:
			if !ok {
				return nil
			}

			err := func() (err error) {
				defer func() {
					if e := recover(); e != nil {
						err = fmt.Errorf("%v", e)
					}
				}()

				return wire.writeMsg(msg)
			}()

			msg.errCh <- err

			if err != nil {
				return err
			}
		}
	}
}
