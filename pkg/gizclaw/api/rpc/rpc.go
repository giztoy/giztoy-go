package rpc

import (
	"context"
	"time"
)

type RPCServer struct {
	Now func() time.Time
}

var _ StrictServerInterface = (*RPCServer)(nil)

func (s *RPCServer) Ping(_ context.Context, _ PingRequest) (*PingResponse, error) {
	now := time.Now
	if s != nil && s.Now != nil {
		now = s.Now
	}
	return &PingResponse{ServerTime: now().UnixMilli()}, nil
}
