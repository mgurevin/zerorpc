package core

import (
	"context"
	"encoding/binary"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	methodAdd  uint64 = 1
	msgAddReq  uint64 = 1
	msgAddResp uint64 = 2
)

type addReq struct {
	NumA int
	NumB int
}

type addResp struct {
	Result int
}

type addReqCodec struct {
	bufPool *sync.Pool
}

func (c *addReqCodec) New() interface{} {
	return &addReq{}
}

func (c *addReqCodec) Encode(dst io.Writer, src interface{}) error {
	buf := c.bufPool.Get().(*[]byte)
	defer c.bufPool.Put(buf)

	b := *buf

	binary.BigEndian.PutUint64(b[:8], uint64(src.(*addReq).NumA))
	binary.BigEndian.PutUint64(b[8:16], uint64(src.(*addReq).NumB))

	_, err := dst.Write(b[:16])

	return err
}

func (c *addReqCodec) Decode(dst interface{}, src io.Reader) error {
	buf := c.bufPool.Get().(*[]byte)
	defer c.bufPool.Put(buf)

	b := *buf

	_, err := io.ReadAtLeast(src, b[:16], 16)
	if err != nil {
		return err
	}

	dst.(*addReq).NumA = int(binary.BigEndian.Uint64(b[:8]))
	dst.(*addReq).NumB = int(binary.BigEndian.Uint64(b[8:16]))

	return nil
}

type addRespCodec struct {
	bufPool *sync.Pool
}

func (c *addRespCodec) New() interface{} {
	return &addResp{}
}

func (c *addRespCodec) Encode(dst io.Writer, src interface{}) error {
	buf := c.bufPool.Get().(*[]byte)
	defer c.bufPool.Put(buf)

	b := *buf

	binary.BigEndian.PutUint64(b[:8], uint64(src.(*addResp).Result))

	_, err := dst.Write(b[:8])

	return err
}

func (c *addRespCodec) Decode(dst interface{}, src io.Reader) error {
	buf := c.bufPool.Get().(*[]byte)
	defer c.bufPool.Put(buf)

	b := *buf

	_, err := io.ReadAtLeast(src, b[:8], 8)
	if err != nil {
		return err
	}

	dst.(*addResp).Result = int(binary.BigEndian.Uint64(b[:8]))

	return nil
}

func BenchmarkZeroRPCCall(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reqCodec := &addReqCodec{
		bufPool: &sync.Pool{New: func() interface{} { buf := make([]byte, 16); return &buf }},
	}

	respCodec := &addRespCodec{
		bufPool: &sync.Pool{New: func() interface{} { buf := make([]byte, 8); return &buf }},
	}

	// a sample RPC call handler
	addService := func(ctx context.Context, req *addReq, res *addResp) error {
		res.Result = req.NumA + req.NumB
		return nil
	}

	handler := func(call Call) error {
		switch call.MethodID() {
		case methodAdd:
			req := call.Message().(*addReq)
			res, err := call.PrepareResponse(msgAddResp)
			if err != nil {
				return err
			}

			err = addService(call.Context(), req, res.(*addResp))

			if err != nil {
				return call.SendError(err)
			}

			return call.SendResponse()
		}

		return nil
	}

	rA := newRPCConn(ctx, handler)
	rB := newRPCConn(ctx, handler)

	rA.RegisterMessageType(msgAddReq, reqCodec)
	rA.RegisterMessageType(msgAddResp, respCodec)

	rB.RegisterMessageType(msgAddReq, reqCodec)
	rB.RegisterMessageType(msgAddResp, respCodec)

	go rA.handleIncomingMsg()
	go rB.handleIncomingMsg()

	for i := 0; i < runtime.NumCPU(); i++ {
		cAr, cBw := io.Pipe()
		cBr, cAw := io.Pipe()

		go func() {
			<-ctx.Done()
			cAr.Close()
			cAw.Close()
			cBr.Close()
			cBw.Close()
		}()

		go rA.handleConn(ctx, cAw, cAr)
		go rB.handleConn(ctx, cBw, cBr)
	}

	b.Run("Call", func(b *testing.B) {
		cnt := int64(0)
		start := time.Now()

		b.ReportAllocs()
		b.SetParallelism(runtime.NumCPU())

		b.StartTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				numA := int(atomic.LoadInt64(&cnt))
				numB := numA + 1

				answerTypeID, answerMsg, err := rA.Call(ctx, methodAdd, msgAddReq, func(msg interface{}) error {
					msg.(*addReq).NumA = numA
					msg.(*addReq).NumB = numB

					return nil
				})

				if err != nil {
					b.Fatal(err)
				}

				if r := answerMsg.(*addResp).Result; r != numA+numB {
					b.Fatalf("sum err: %d + %d = %d (must %d)", numA, numB, r, numA+numB)
				}

				if err = rA.ReleaseCodecMsg(answerTypeID, answerMsg); err != nil {
					b.Fatal(err)
				}

				atomic.AddInt64(&cnt, 1)
			}
		})
		b.StopTimer()

		b.ReportMetric(float64(cnt)/time.Since(start).Seconds(), "call/sec")
	})
}

func BenchmarkZeroRPCPush(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reqCodec := &addReqCodec{
		bufPool: &sync.Pool{New: func() interface{} { buf := make([]byte, 16); return &buf }},
	}

	// a sample RPC push handler
	addService := func(ctx context.Context, req *addReq) {
		if req.NumA != 2 {
			panic("err")
		}

		if req.NumB != 2 {
			panic("err")
		}
	}

	handler := func(call Call) error {
		switch call.MethodID() {
		case methodAdd:
			req := call.Message().(*addReq)

			addService(call.Context(), req)
		}

		return nil
	}

	rA := newRPCConn(ctx, handler)
	rB := newRPCConn(ctx, handler)

	rA.RegisterMessageType(msgAddReq, reqCodec)
	rB.RegisterMessageType(msgAddReq, reqCodec)

	go rA.handleIncomingMsg()
	go rB.handleIncomingMsg()

	for i := 0; i < runtime.NumCPU(); i++ {
		cAr, cBw := io.Pipe()
		cBr, cAw := io.Pipe()

		go func() {
			<-ctx.Done()
			cAr.Close()
			cAw.Close()
			cBr.Close()
			cBw.Close()
		}()

		go rA.handleConn(ctx, cAw, cAr)
		go rB.handleConn(ctx, cBw, cBr)
	}

	b.Run("Push", func(b *testing.B) {
		cnt := int64(0)
		start := time.Now()

		b.ReportAllocs()
		b.SetParallelism(runtime.NumCPU())

		b.StartTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				err := rA.Push(ctx, methodAdd, msgAddReq, func(msg interface{}) error {
					msg.(*addReq).NumA = 2
					msg.(*addReq).NumB = 2

					return nil
				})

				if err != nil {
					b.Fatal(err)
				}

				atomic.AddInt64(&cnt, 1)
			}
		})
		b.StopTimer()

		b.ReportMetric(float64(cnt)/time.Since(start).Seconds(), "push/sec")
	})
}
