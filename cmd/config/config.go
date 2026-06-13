package config

import (
	"log/slog"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

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
