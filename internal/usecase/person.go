package usecase

import (
	"context"
	"fmt"
	"person-grpc/internal/domain"
	"person-grpc/internal/ports"

	"github.com/google/uuid"
)

type PersonUseCaseImplement struct {
	repo     ports.PersonRepository
	enricher ports.PersonEnricher
}

func NewPersonUseCase(repo ports.PersonRepository, enricher ports.PersonEnricher) *PersonUseCaseImplement {
	return &PersonUseCaseImplement{
		repo:     repo,
		enricher: enricher,
	}
}

func (uc *PersonUseCaseImplement) CreatePerson(ctx context.Context, firstName, lastName string, patronymic *string, emails []string) (*domain.Person, error) {
	if firstName == "" || lastName == "" {
		return nil, fmt.Errorf("%w: first name AND last name are required", domain.ErrInvalidData)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", domain.ErrInvalidData)
	}

	age, gender, nationality, err := uc.enricher.Enrich(ctx, fmt.Sprintf("%s %s", firstName, lastName))
	if err != nil {
		// todo: определиться с поведением. Склонен записать в лог и пропустить полученные данные / данные по умолчанию
		return nil, fmt.Errorf("failed to enrich person data: %w", err)
	}

	person := &domain.Person{
		ID:          id,
		FirstName:   firstName,
		LastName:    lastName,
		Patronymic:  patronymic,
		Age:         age,
		Gender:      gender,
		Nationality: nationality,
		Emails:      emails,
	}

	if err := uc.repo.Save(ctx, person); err != nil {
		return nil, fmt.Errorf("failed to save person: %w", err)
	}

	return person, nil
}

func (uc *PersonUseCaseImplement) GetPerson(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: id required", domain.ErrInvalidData)
	}

	return uc.repo.GetByID(ctx, id)
}

func (uc *PersonUseCaseImplement) ListPersons(ctx context.Context, page, limit uint32) ([]*domain.Person, uint32, uint32, uint32, error) {
	const (
		DefaultLimit = 10
		MaxLimit     = 100
	)
	if page == 0 {
		page = 1
	}

	if limit == 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit

		// todo: определиться. Можно сделать
		//return nil, 0, fmt.Errorf("%w: limit cannot exceed %d", domain.ErrInvalidData, MaxLimit)
	}

	offset := (page - 1) * limit

	return uc.repo.List(ctx, offset, limit)
}

func (uc *PersonUseCaseImplement) UpdatePerson(ctx context.Context, id uuid.UUID, input ports.UpdatePersonInput) (*domain.Person, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: id required", domain.ErrInvalidData)
	}

	person, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get specified person: %w", err)
	}

	if input.FirstName != nil {
		person.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		person.LastName = *input.LastName
	}
	if input.Patronymic != nil {
		person.Patronymic = input.Patronymic
	}
	if input.Age != nil {
		person.Age = *input.Age
	}
	if input.Gender != nil {
		person.Gender = *input.Gender
	}
	if input.Nationality != nil {
		person.Nationality = *input.Nationality
	}
	if input.Emails != nil {
		person.Emails = *input.Emails
	}

	if err := uc.repo.Update(ctx, person); err != nil {
		return nil, fmt.Errorf("failed to update person: %w", err)
	}

	return person, nil
}

func (uc *PersonUseCaseImplement) DeletePerson(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("%w: id required", domain.ErrInvalidData)
	}

	return uc.repo.Delete(ctx, id)
}
