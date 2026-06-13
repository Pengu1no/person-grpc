package grpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		timeStart := time.Now()

		resp, err := handler(ctx, req)

		status_, _ := status.FromError(err)
		code := status_.Code().String()

		reqDuration := time.Since(timeStart)
		reqMethod := info.FullMethod

		reqLogger := logger.With(
			slog.String("method", reqMethod),
			slog.String("code", code),
			slog.Duration("duration", reqDuration),
		)

		if err != nil {
			reqLogger.Error("Request processing failed", slog.String("error", err.Error()))
		} else {
			reqLogger.Info("OK")
		}

		return resp, err
	}
}
