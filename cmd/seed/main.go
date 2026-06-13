package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"person-grpc/cmd/config"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type seedPerson struct {
	id          string
	firstName   string
	lastName    string
	patronymic  *string
	age         int
	gender      int
	nationality string
	emails      []string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.InitCfg()

	if cfg.Environment == "production" {
		logger.Error("Cannot start seeding in production mode. Shutting down...")
		os.Exit(1)
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.DBUsername, cfg.DBPassword, cfg.DBAddress, cfg.DBPort, cfg.DBName)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pgxConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		logger.Error("Failed to parse postgres connection string", slog.Any("err", err))
		os.Exit(1)
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		logger.Error("Failed to connect to database", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("Failed to ping database", slog.Any("err", err))
		os.Exit(1)
	}

	logger.Debug("Successfully connected to database. Starting seeding", slog.String("db_name", cfg.DBName))

	patronymic0 := "Васильевич"
	patronymic1 := "Васильевна"
	patronymic2 := "Дмитриевич"

	seedData := []seedPerson{
		{"019ec1a9-0f6f-7d39-9dd4-3d98e722f36c", "Дмитрий", "Верещагин", &patronymic0, 38, 1, "RU", []string{"dima@mail.ru"}},
		{"019ec1a9-0f6f-7d3a-9042-029490c95bb8", "Ольга", "Назарова", &patronymic1, 22, 2, "RU", []string{"olga.work@company.com", "nazarova.o@yandex.ru"}},
		{"019ec1a9-0f6f-7d3b-ac25-91e9534bea6a", "Jane", "Doe", nil, 31, 2, "US", []string{"janedoe@gmail.com"}},
		{"019ec1a9-0f6f-7d3c-b826-2b6b266bc642", "Juan", "Xavier", nil, 57, 1, "BR", []string{}},
		{"019ec1a9-0f6f-7d3d-8bff-86ab77cf555f", "Петр", "Ковалев", &patronymic2, 31, 1, "BY", []string{"kovalev.petr@yandex.ru", "kovalev.petr@mail.ru", "kovalev.petr@gmail.com"}},
		{"019ec1a9-0f6f-7d3e-8f6a-9a7dd4da3b85", "Xi", "Li", nil, 42, 1, "CN", []string{}},
		{"019ec1a9-0f6f-7d3f-a1b4-9eb18e22061b", "John", "Singer", nil, 26, 1, "US", []string{"john@yahoo.com"}},
		{"019ec1a9-0f6f-7d40-ae2d-0a878f7f28f3", "Paula", "Rodriguez", nil, 31, 2, "ES", []string{"rodriguezp@gmail.com"}},
		{"019ec1a9-0f6f-7d41-905b-a9e07737179e", "Василий", "Иванов", &patronymic2, 30, 1, "RU", []string{"vasily_ivanov@mail.ru"}},
		{"019ec1a9-0f6f-7d42-9e12-9769544c6efb", "Hans", "Muller", nil, 50, 1, "DE", []string{"hmul@gmail.com", "de.hans@yahoo.com"}},
	}

	for _, person := range seedData {
		pID, err := uuid.Parse(person.id)
		if err != nil {
			logger.Error("Invalid hardcoded UUID", slog.String("id", person.id))
			continue
		}

		err = func() error {
			tx, e := pool.Begin(ctx)
			if e != nil {
				return e
			}
			defer tx.Rollback(ctx)

			q := `
INSERT INTO persons (id, first_name, last_name, patronymic, age, gender, nationality)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO NOTHING`
			_, e = tx.Exec(ctx, q, pID, person.firstName, person.lastName, person.patronymic, person.age, person.gender, person.nationality)
			if e != nil {
				return fmt.Errorf("could not insert person: %w", e)
			}

			for _, email := range person.emails {
				q = `
INSERT INTO person_emails (person_id, email)
VALUES ($1, $2)
ON CONFLICT (person_id, email) DO NOTHING`
				_, e = tx.Exec(ctx, q, pID, email)
				if e != nil {
					return fmt.Errorf("could not insert email %s: %w", email, e)
				}
			}
			return tx.Commit(ctx)
		}()

		if err != nil {
			slog.Error("Failed to seed record", slog.String("id", person.id), slog.Any("error", err))
		}
	}

	logger.Info("Database seeding finished")
}
