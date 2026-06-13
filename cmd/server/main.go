package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"person-grpc/internal/adapters/db"
	grpcAdapter "person-grpc/internal/adapters/grpc"
	httpClient "person-grpc/internal/adapters/http-client"
	"person-grpc/internal/usecase"
	"person-grpc/pkg/personpb"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// todo: добавить example env файл в репо

type Config struct {
	DBAddress                 string `env:"DB_ADDRESS"`
	DBPort                    int    `env:"DB_PORT"`
	DBUsername                string `env:"DB_USER"`
	DBPassword                string `env:"DB_PASSWORD"`
	DBName                    string `env:"DB_NAME"`
	GRPCPort                  int    `env:"GRPC_PORT" envDefault:"8080"`
	ExternalAPITimeoutSeconds int    `env:"EXTERNAL_API_TIMEOUT_SECONDS" envDefault:"5"`
	ShutdownTimeoutSeconds    int    `env:"SHUTDOWN_TIMEOUT_SECONDS" envDefault:"15"`
	LogLevel                  int    `env:"LOG_LEVEL" envDefault:"0"`
	Environment               string `env:"ENVIRONMENT" envDefault:"development"`

	// LogLevel = [-4]Debug | [0]Info | [4]Warn | [8]Error
}

func InitCfg() Config {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := godotenv.Load(); err != nil {
		logger.Warn("Error loading .env file. Attempting to load system env vars.")
	}

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		logger.Error("Failed to parse env into Config struct")
		os.Exit(1)
	}

	logger.Info("Config loaded successfully")

	return cfg
}

func main() {
	cfg := InitCfg()

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

	// todo: провести отладку с замерами быстродействия
	dataEnricher := httpClient.NewParallelEnricher(agify, genderize, nationalize)

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
