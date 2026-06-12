package ports

import (
	"context"
	"person-grpc/internal/domain"

	"github.com/google/uuid"
)

type PersonRepository interface {
	Save(ctx context.Context, person *domain.Person) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Person, error)
	List(ctx context.Context, offset, limit uint32) ([]*domain.Person, uint32, uint32, uint32, error)
	Update(ctx context.Context, person *domain.Person) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type AgeProvider interface {
	GetAge(ctx context.Context, fullName string) (uint32, error)
}

type GenderProvider interface {
	GetGender(ctx context.Context, fullName string) (domain.Gender, error)
}

type NationalityProvider interface {
	GetNationality(ctx context.Context, fullName string) (string, error)
}

type PersonEnricher interface {
	Enrich(ctx context.Context, fullName string) (age uint32, gender domain.Gender, nationality string, err error)
}
