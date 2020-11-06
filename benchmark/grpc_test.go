package benchmark

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative service.proto

import (
	context "context"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
)

type sumService struct {
	UnimplementedSumServer
}

func (s *sumService) Sum(ctx context.Context, req *SumRequest) (*SumResponse, error) {
	return &SumResponse{Result: req.NumA + req.NumB}, nil
}

type pipeListener struct {
	connCh chan net.Conn
}

func (l *pipeListener) Accept() (net.Conn, error) {
	return <-l.connCh, nil
}

func (l *pipeListener) Close() error {
	return nil
}

func (l *pipeListener) Addr() net.Addr {
	return nil
}

type pipeConn struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (c *pipeConn) Read(b []byte) (n int, err error)   { return c.r.Read(b) }
func (c *pipeConn) Write(b []byte) (n int, err error)  { return c.w.Write(b) }
func (c *pipeConn) Close() error                       { c.r.Close(); c.w.Close(); return nil }
func (c *pipeConn) LocalAddr() net.Addr                { return nil }
func (c *pipeConn) RemoteAddr() net.Addr               { return nil }
func (c *pipeConn) SetDeadline(t time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(t time.Time) error { return nil }

func BenchmarkGRPCCall(b *testing.B) {
	sumSvc := &sumService{}

	grpcSrv := grpc.NewServer()

	RegisterSumServer(grpcSrv, sumSvc)

	connCh := make(chan net.Conn)

	go func() {
		if err := grpcSrv.Serve(&pipeListener{connCh}); err != nil {
			fmt.Printf("grpc server error: %v\n", err)
		}
	}()

	sumClients := make([]grpc.ClientConnInterface, 0, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		srvR, clientW := io.Pipe()
		clientR, srvW := io.Pipe()

		srvConn, clientConn := &pipeConn{srvR, srvW}, &pipeConn{clientR, clientW}

		grpcClientConn, err := grpc.Dial("", grpc.WithInsecure(), grpc.WithDialer(func(string, time.Duration) (net.Conn, error) {
			return clientConn, nil
		}))
		if err != nil {
			b.Fatal(err)
		}

		connCh <- srvConn

		sumClients = append(sumClients, grpcClientConn)
	}

	reqPool := sync.Pool{New: func() interface{} { return &SumRequest{} }}
	respPool := sync.Pool{New: func() interface{} { return &SumResponse{} }}

	b.Run("Call", func(b *testing.B) {
		cnt := int64(0)
		start := time.Now()

		b.ReportAllocs()
		b.SetParallelism(runtime.NumCPU())

		b.StartTimer()
		b.RunParallel(func(pb *testing.PB) {
			clients := len(sumClients)
			ctx := context.Background()

			for pb.Next() {
				numA := atomic.LoadInt64(&cnt)
				numB := numA + 1

				req := reqPool.Get().(*SumRequest)
				req.NumA = numA
				req.NumB = numB

				resp := respPool.Get().(*SumResponse)

				err := sumClients[int(numA)%clients].Invoke(ctx, "/Sum/Sum", req, resp)
				if err != nil {
					b.Fatal(err)
				}

				if r := resp.Result; r != numA+numB {
					b.Fatalf("sum err: %d + %d = %d (must %d)", numA, numB, r, numA+numB)
				}

				reqPool.Put(req)
				respPool.Put(resp)

				atomic.AddInt64(&cnt, 1)
			}
		})
		b.StopTimer()

		b.ReportMetric(float64(cnt)/time.Since(start).Seconds(), "call/sec")
	})
}
