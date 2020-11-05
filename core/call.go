package core

import (
	"context"
	"sync/atomic"
)

func (p *rpcConn) Push(ctx context.Context, methodID, msgTypeID uint64, msgFunc MessageFunc) (err error) {
	m := p.getConnMsg()

	m.ctx = ctx
	m.op = rpcOpPush
	m.method = methodID
	m.msgType = msgTypeID

	m.msgCodec, err = p.findCodec(msgTypeID)
	if err != nil {
		p.putConnMsg(m)
		return err
	}

	m.msg, err = m.msgCodec.getMsg()
	if err != nil {
		p.putConnMsg(m)
		return err
	}

	defer m.msgCodec.putMsg(m.msg)

	if err = msgFunc(m.msg); err != nil {
		p.putConnMsg(m)
		return err
	}

	select {
	case <-ctx.Done():
		p.putConnMsg(m)

		return ctx.Err()

	case <-p.ctx.Done():
		p.putConnMsg(m)

		return p.ctx.Err()

	case p.sendCh <- m:
		defer p.putConnMsg(m)

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-p.ctx.Done():
			return p.ctx.Err()

		case err := <-m.errCh:
			return err
		}
	}
}

func (p *rpcConn) Call(ctx context.Context, methodID, msgTypeID uint64, msgFunc MessageFunc) (answerType uint64, answer interface{}, err error) {
	m := p.getConnMsg()

	m.ctx = ctx
	m.op = rpcOpCall
	m.callID = atomic.AddUint64(&p.seq, uint64(1))
	m.method = methodID
	m.msgType = msgTypeID

	m.msgCodec, err = p.findCodec(msgTypeID)
	if err != nil {
		p.putConnMsg(m)
		return 0, nil, err
	}

	m.msg, err = m.msgCodec.getMsg()
	if err != nil {
		p.putConnMsg(m)
		return 0, nil, err
	}

	defer m.msgCodec.putMsg(m.msg)

	if err = msgFunc(m.msg); err != nil {
		p.putConnMsg(m)
		return 0, nil, err
	}

	p.waitMu.Lock()
	p.wait[m.callID] = m.replyCh
	p.waitMu.Unlock()

	defer func(callID uint64) {
		p.waitMu.Lock()
		defer p.waitMu.Unlock()

		delete(p.wait, callID)
	}(m.callID)

	select {
	case <-ctx.Done():
		p.putConnMsg(m)

		return 0, nil, ctx.Err()

	case <-p.ctx.Done():
		p.putConnMsg(m)

		return 0, nil, p.ctx.Err()

	case p.sendCh <- m:
		select {
		case <-ctx.Done():
			p.putConnMsg(m)

			return 0, nil, ctx.Err()

		case <-p.ctx.Done():
			p.putConnMsg(m)

			return 0, nil, p.ctx.Err()

		case err := <-m.errCh:
			defer p.putConnMsg(m)

			if err != nil {
				return 0, nil, err
			}

			select {
			case <-ctx.Done():
				return 0, nil, ctx.Err()

			case <-p.ctx.Done():
				return 0, nil, p.ctx.Err()

			case reply := <-m.replyCh:
				defer p.putConnMsg(reply)

				switch reply.op {
				case rpcOpRet:
					return reply.msgType, reply.msg, nil

				case rpcOpErrUnimplemented:
					return 0, nil, ErrUnimplementedRPC

				case rpcOpErr:
					return 0, nil, reply.msg.(error)

				default:
					panic("here is a bug")
				}
			}
		}
	}
}
