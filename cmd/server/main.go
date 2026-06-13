package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"person-grpc/cmd/config"
	"person-grpc/internal/adapters/db"
	grpcAdapter "person-grpc/internal/adapters/grpc"
	httpClient "person-grpc/internal/adapters/http-client"
	"person-grpc/internal/usecase"
	"person-grpc/pkg/personpb"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.InitCfg()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.Level(cfg.LogLevel),
	}))
	slog.SetDefault(logger)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.DBUsername, cfg.DBPassword, cfg.DBAddress, cfg.DBPort, cfg.DBName)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pgxConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		slog.Error("Failed to parse postgres connection string", slog.Any("err", err))
		os.Exit(1)
	}

	//pgxConfig.MaxConnIdleTime = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		slog.Error("Failed to connect to database", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("Failed to ping database", slog.Any("err", err))
		os.Exit(1)
	}

	slog.Debug("Successfully connected to database", slog.String("db_name", cfg.DBName))

	apiTimeout := time.Duration(cfg.ExternalAPITimeoutSeconds) * time.Second

	agify := httpClient.NewAgifyClient(apiTimeout)
	genderize := httpClient.NewGenderizeClient(apiTimeout)
	nationalize := httpClient.NewNationalizeClient(apiTimeout)

	dataEnricher := httpClient.NewParallelEnricher(agify, genderize, nationalize, cfg.ExternalAPIMaxConcurrent)

	repo := db.NewPostgresRepository(pool)

	useCase := usecase.NewPersonUseCase(repo, dataEnricher)

	handler := grpcAdapter.NewPersonHandler(useCase)

	grpcPort := fmt.Sprintf(":%d", cfg.GRPCPort)
	listen, err := net.Listen("tcp", grpcPort)
	if err != nil {
		slog.Error("Failed to listen", slog.Any("err", err))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcAdapter.LoggingInterceptor(logger)),
	)
	personpb.RegisterPersonServiceServer(grpcServer, handler)
	if cfg.Environment != "production" {
		reflection.Register(grpcServer)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		slog.Info("Starting gRPC server", slog.String("port", grpcPort))
		if err := grpcServer.Serve(listen); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			slog.Error("Failed to start gRPC server", slog.Any("err", err))
			os.Exit(1)
		}
	}()

	sig := <-stop
	slog.Info("Stopping gRPC server", slog.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeoutSeconds)*time.Second)
	defer shutdownCancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		slog.Info("Graceful shutdown complete")
	case <-shutdownCtx.Done():
		slog.Info("Graceful timeout exceeded. Performing forced shutdown")
		grpcServer.Stop()
	}

	slog.Info("Service shutdown complete")
}
