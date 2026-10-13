package internalgrpc

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Logger interface{ Info(string, ...any) }

func loggingInterceptor(log Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		response, err := handler(ctx, request)
		ip := "unknown"
		if client, ok := peer.FromContext(ctx); ok && client.Addr != nil {
			ip = client.Addr.String()
			if host, _, splitErr := net.SplitHostPort(ip); splitErr == nil {
				ip = host
			}
		}
		agent := ""
		if meta, ok := metadata.FromIncomingContext(ctx); ok {
			if values := meta.Get("user-agent"); len(values) > 0 {
				agent = values[0]
			}
		}
		log.Info("grpc request", "client_ip", ip, "request_time", start.Format(time.RFC3339Nano),
			"method", info.FullMethod, "protocol", "gRPC", "status", status.Code(err).String(),
			"latency", time.Since(start), "user_agent", agent)
		return response, err
	}
}
