package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
}

func InitCfg() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env file. Attempting read from system env")
	}

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("Failed to parse env into Config struct")
	}

	log.Println("Config loaded successfully")

	return cfg
}

func main() {
	cfg := InitCfg()

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.DBUsername, cfg.DBPassword, cfg.DBAddress, cfg.DBPort, cfg.DBName)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pgxConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("failed to parse postgres connection string: %v", err)
	}

	//pgxConfig.MaxConnIdleTime = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("Successfully connected to database")

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
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	personpb.RegisterPersonServiceServer(grpcServer, handler)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Printf("gRPC server is listening on port %s", grpcPort)
		if err := grpcServer.Serve(listen); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("failed to serve gRPC server: %v", err)
		}
	}()

	sig := <-stop
	log.Printf("received signal: %v. Shutting down..", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeoutSeconds)*time.Second)
	defer shutdownCancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Printf("graceful shutdown complete")
	case <-shutdownCtx.Done():
		log.Printf("[INFO] graceful timeout exceeded. Performing forced shutdown")
		grpcServer.Stop()
	}

	log.Printf("Service shutdown complete")
}
