package grpcpool

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/moyrne/motool/pool"
)

func NewGrpc(expire time.Duration, opts ...grpc.DialOption) pool.Pool[*grpc.ClientConn] {
	return pool.New(
		expire,
		&grpcAdaptor{opts: opts},
	)
}

var _ pool.Adaptor[*grpc.ClientConn] = (*grpcAdaptor)(nil)

type grpcAdaptor struct {
	opts []grpc.DialOption
}

func (g *grpcAdaptor) New(ctx context.Context, target string) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, target, g.opts...)
}

func (g *grpcAdaptor) Check(c *grpc.ClientConn) error {
	if c.GetState() == connectivity.Shutdown {
		return errors.New("connection closed")
	}
	return nil
}

func (g *grpcAdaptor) Close(c *grpc.ClientConn) error {
	return c.Close()
}
