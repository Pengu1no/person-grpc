package db

import (
	"context"
	"errors"
	"fmt"
	"person-grpc/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool}
}

/**
Util section
*/

func (r *PostgresRepository) getPersonEmails(ctx context.Context, id uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, "SELECT email FROM person_emails WHERE person_id = $1", id)
	if err != nil {
		return nil, fmt.Errorf("failed to query email slice: %w", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}
		emails = append(emails, email)
	}
	return emails, nil
}

func (r *PostgresRepository) populateEmails(ctx context.Context, persons []*domain.Person) error {
	ids := make([]uuid.UUID, len(persons))
	personMap := make(map[uuid.UUID]*domain.Person)
	for i, person := range persons {
		ids[i] = person.ID
		personMap[person.ID] = person
		person.Emails = []string{}
	}

	q := "SELECT person_id, email FROM person_emails WHERE person_id = ANY($1)"
	rows, err := r.pool.Query(ctx, q, ids)
	if err != nil {
		return fmt.Errorf("failed to query email slice: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pid uuid.UUID
		var email string
		if err := rows.Scan(&pid, &email); err != nil {
			return fmt.Errorf("failed to scan email: %w", err)
		}
		if p, ok := personMap[pid]; ok {
			p.Emails = append(p.Emails, email)
		}
	}

	return nil
}

/**
Main section
*/

func (r *PostgresRepository) Save(ctx context.Context, person *domain.Person) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `
INSERT INTO persons (id, first_name, last_name, patronymic, age, gender, nationality) 
VALUES ($1, $2, $3, $4, $5, $6, $7);
`
	_, err = tx.Exec(ctx, q, person.ID, person.FirstName, person.LastName, person.Patronymic, person.Age, person.Gender, person.Nationality)
	if err != nil {
		return fmt.Errorf("failed to insert person: %w", err)
	}

	if len(person.Emails) > 0 {
		for _, email := range person.Emails {
			emailQuery := `INSERT INTO person_emails (person_id, email) VALUES ($1, $2)`
			_, err = tx.Exec(ctx, emailQuery, person.ID, email)
			if err != nil {
				return fmt.Errorf("failed to insert email %s: %w", email, err)
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
	q := `
SELECT id, first_name, last_name, patronymic, age, gender, nationality
FROM persons WHERE id = $1`
	var person domain.Person
	var genderRaw int32

	err := r.pool.QueryRow(ctx, q, id).Scan(
		&person.ID,
		&person.FirstName,
		&person.LastName,
		&person.Patronymic,
		&person.Age,
		&genderRaw,
		&person.Nationality)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPersonNotFound
		}
		return nil, fmt.Errorf("failed to query person: %w", err)
	}
	person.Gender = domain.Gender(genderRaw)
	person.Emails, err = r.getPersonEmails(ctx, id)
	if err != nil {
		return nil, err
	}

	return &person, nil
}

// List returns { []Person on page, total rows num, error }
// populates all persons on selected page + all associated emails then merges data on person.id
func (r *PostgresRepository) List(ctx context.Context, offset, limit uint32) ([]*domain.Person, uint32, error) {
	var totalRows uint32
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM persons").Scan(&totalRows)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count persons: %w", err)
	}

	if totalRows == 0 {
		return []*domain.Person{}, 0, nil
	}

	q := `
SELECT id, first_name, last_name, patronymic, age, gender, nationality
FROM persons
ORDER BY id DESC
LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query persons: %w", err)
	}
	defer rows.Close()

	var persons []*domain.Person
	for rows.Next() {
		var p domain.Person
		var genderRaw int32
		if err := rows.Scan(
			&p.ID,
			&p.FirstName,
			&p.LastName,
			&p.Patronymic,
			&p.Age,
			&genderRaw,
			&p.Nationality); err != nil {
			return nil, 0, fmt.Errorf("failed to scan person: %w", err)
		}
		p.Gender = domain.Gender(genderRaw)
		persons = append(persons, &p)
	}

	if len(persons) > 0 {
		if err := r.populateEmails(ctx, persons); err != nil {
			return nil, 0, err
		}
	}

	return persons, totalRows, nil
}

func (r *PostgresRepository) Update(ctx context.Context, person *domain.Person) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `
UPDATE persons
SET first_name = $1, last_name = $2, patronymic = $3, age = $4, gender = $5, nationality = $6 
WHERE id = $7`
	_, err = tx.Exec(
		ctx, q,
		person.FirstName,
		person.LastName,
		person.Patronymic,
		person.Age,
		person.Gender,
		person.Nationality,
		person.ID)
	if err != nil {
		return fmt.Errorf("failed to update person: %w", err)
	}

	if len(person.Emails) == 0 {
		_, err = tx.Exec(ctx, "DELETE FROM person_emails WHERE person_id = $1", person.ID)
		if err != nil {
			return fmt.Errorf("failed to clear emails: %w", err)
		}
	} else {
		q = `
DELETE FROM person_emails
WHERE person_id = $1 AND NOT (email = ANY($2))`

		_, err = tx.Exec(ctx, q, person.ID, person.Emails)
		if err != nil {
			return fmt.Errorf("failed to delete removed emails: %w", err)
		}

		q = `
INSERT INTO person_emails (person_id, email) 
VALUES ($1, $2)
ON CONFLICT (person_id, email) DO NOTHING`
		for _, email := range person.Emails {
			_, err = tx.Exec(ctx, q, person.ID, email)
			if err != nil {
				return fmt.Errorf("failed to upsert email %s: %w", email, err)
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	q := `DELETE FROM persons WHERE id = $1`
	res, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("failed to delete person: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrPersonNotFound
	}
	return nil
}
