package ports

import (
	"context"
	"person-grpc/internal/domain"

	"github.com/google/uuid"
)

type UpdatePersonInput struct {
	FirstName   *string
	LastName    *string
	Patronymic  *string
	Age         *uint32
	Gender      *domain.Gender
	Nationality *string
	Emails      *[]string
}

type PersonUseCase interface {
	CreatePerson(ctx context.Context, firstName, lastName string, patronymic *string, emails []string) (*domain.Person, error)
	GetPerson(ctx context.Context, id uuid.UUID) (*domain.Person, error)
	ListPersons(ctx context.Context, page, limit uint32) ([]*domain.Person, uint32, uint32, uint32, error)
	UpdatePerson(ctx context.Context, id uuid.UUID, input UpdatePersonInput) (*domain.Person, error)
	DeletePerson(ctx context.Context, id uuid.UUID) error
}
