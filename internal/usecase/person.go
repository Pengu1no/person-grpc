package usecase

import (
	"context"
	"fmt"
	"log/slog"
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

	fullName := fmt.Sprintf("%s %s", firstName, lastName)

	// todo: можем завернуть запрос при наличии хотя бы одного невалидного адреса
	//for _, email := range emails {
	//	if err := validateEmail(email); err != nil {
	//		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidData, err.Error())
	//	}
	//}
	// todo: а можем просто написать об этом в лог, убрать невалидный из списка и пойти дальше
	// предпочту этот вариант
	checkedEmails := make([]string, 0, len(emails))
	for _, email := range emails {
		if err := validateEmail(email); err == nil {
			checkedEmails = append(checkedEmails, email)
		} else {
			slog.Warn("Invalid email address. Skipping", slog.String("id", id.String()), slog.String("email", email))
		}
	}
	emails = checkedEmails

	age, gender, nationality, err := uc.enricher.Enrich(ctx, fullName)
	if err != nil {
		// todo: определиться с поведением. Склонен записать в лог и пропустить полученные данные / данные по умолчанию
		// todo: но можно и так
		// return nil, fmt.Errorf("failed to enrich person data: %w", err)

		slog.Warn("Enrichment partial failure. Will use zero values", slog.String("id", id.String()), slog.String("full_name", fullName), slog.Any("err", err))
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

	data, total, err := uc.repo.List(ctx, offset, limit)
	return data, total, page, limit, err
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
		if err := validateAge(*input.Age); err != nil {
			return nil, fmt.Errorf("%w: %s", domain.ErrInvalidData, err.Error())
		}
		person.Age = *input.Age
	}
	if input.Gender != nil {
		person.Gender = *input.Gender
	}
	if input.Nationality != nil {
		person.Nationality = *input.Nationality
	}
	if input.Emails != nil {
		checkedEmails := make([]string, 0, len(*input.Emails))
		for _, email := range *input.Emails {
			if err := validateEmail(email); err == nil {
				checkedEmails = append(checkedEmails, email)
			} else {
				slog.Warn("Invalid email address. Skipping", slog.String("id", id.String()), slog.String("email", email))
			}
		}

		person.Emails = checkedEmails
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
