package usecase_test

import (
	"context"
	"errors"
	"person-grpc/internal/domain"
	"person-grpc/internal/ports"
	"person-grpc/internal/usecase"
	"testing"

	"github.com/google/uuid"
)

type mockRepo struct {
	saveFunc    func(ctx context.Context, person *domain.Person) error
	getByIdFunc func(ctx context.Context, id uuid.UUID) (*domain.Person, error)
	listFunc    func(ctx context.Context, offset, limit uint32) ([]*domain.Person, uint32, error)
	updateFunc  func(ctx context.Context, person *domain.Person) error
	deleteFunc  func(ctx context.Context, id uuid.UUID) error
}

func (r *mockRepo) Save(ctx context.Context, person *domain.Person) error {
	return r.saveFunc(ctx, person)
}
func (r *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
	return r.getByIdFunc(ctx, id)
}
func (r *mockRepo) List(ctx context.Context, offset, limit uint32) ([]*domain.Person, uint32, error) {
	return r.listFunc(ctx, offset, limit)
}
func (r *mockRepo) Update(ctx context.Context, person *domain.Person) error {
	return r.updateFunc(ctx, person)
}
func (r *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.deleteFunc(ctx, id)
}

type mockEnricher struct {
	enrichFunc func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error)
}

func (e *mockEnricher) Enrich(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
	return e.enrichFunc(ctx, fullName)
}

func TestPersonServiceCreate_Success(t *testing.T) {
	repo := &mockRepo{
		saveFunc: func(ctx context.Context, person *domain.Person) error {
			if person.FirstName != "Hans" {
				t.Errorf("FirstName is %s, want Hans", person.FirstName)
			}
			return nil
		},
	}

	enricher := &mockEnricher{
		enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
			return 50, domain.GenderMale, "DE", nil
		},
	}

	useCase := usecase.NewPersonUseCase(repo, enricher)
	person, e := useCase.CreatePerson(context.Background(), "Hans", "Muller", nil, []string{"hans@test.com"})

	if e != nil {
		t.Fatal("Unexpected error", e)
	}

	if person.Age != 50 {
		t.Errorf("Age is %d, want 50", person.Age)
	}
	if person.Gender != domain.GenderMale {
		t.Errorf("Gender is %d, want Male (1)", person.Gender)
	}
	if person.Nationality != "DE" {
		t.Errorf("Nationality is %s, want DE", person.Nationality)
	}
}

func TestPersonServiceCreate_InvalidEmail(t *testing.T) {
	repo := &mockRepo{saveFunc: func(ctx context.Context, person *domain.Person) error { return nil }}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}
	useCase := usecase.NewPersonUseCase(repo, enricher)

	p, e := useCase.CreatePerson(context.Background(), "Василий", "Васильев", nil, []string{
		"email-invalid@test",
		"email-invalid",
		"@mail.ru",
		"ivan@m.c",
		"valid@test.com",
	})

	if e != nil {
		t.Error("Unexpected error. Person should've been created", e)
	}

	if len(p.Emails) > 1 {
		t.Errorf("Email count is %d, want 1", len(p.Emails))
	}
	if p.Emails[0] != "valid@test.com" {
		t.Errorf("Unexpected valid email address %s, want `valid@test.com`", p.Emails[0])
	}
}

func TestPersonServiceCreate_NoFullName(t *testing.T) {
	repo := &mockRepo{saveFunc: func(ctx context.Context, person *domain.Person) error { return nil }}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}
	useCase := usecase.NewPersonUseCase(repo, enricher)

	_, e := useCase.CreatePerson(context.Background(), "Hans", "", nil, []string{})
	if e == nil {
		t.Error("Expected error, got none")
	}
	if !errors.Is(e, domain.ErrInvalidData) {
		t.Error("Expected", domain.ErrInvalidData, "got", e)
	}
}

func TestPersonServiceGetByID_Success(t *testing.T) {
	targetId, _ := uuid.NewV7()
	target := domain.Person{ID: targetId, FirstName: "Hans", LastName: "Muller"}

	repo := &mockRepo{getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
		return &target, nil
	}}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}

	useCase := usecase.NewPersonUseCase(repo, enricher)
	person, e := useCase.GetPerson(context.Background(), targetId)

	if e != nil {
		t.Error("Unexpected error", e)
	}
	if person.ID != targetId || person.FirstName != "Hans" || person.LastName != "Muller" {
		t.Errorf("Got %+v, expected %+v", person, target)
	}
}

func TestPersonServiceGetByID_InvalidUUID(t *testing.T) {
	repo := &mockRepo{getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
		return nil, domain.ErrPersonNotFound
	}}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}
	useCase := usecase.NewPersonUseCase(repo, enricher)

	_, e := useCase.GetPerson(context.Background(), uuid.Nil)
	if e == nil {
		t.Error("Expected error, got none")
	}
	if !errors.Is(e, domain.ErrInvalidData) {
		t.Error("Expected", domain.ErrInvalidData, "got", e)
	}
}

func TestPersonServiceGetByID_PersonNotFound(t *testing.T) {
	repo := &mockRepo{getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
		return nil, domain.ErrPersonNotFound
	}}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}
	useCase := usecase.NewPersonUseCase(repo, enricher)

	_, e := useCase.GetPerson(context.Background(), uuid.New())

	if e == nil {
		t.Error("Expected error, got none")
	}
	if !errors.Is(e, domain.ErrPersonNotFound) {
		t.Error("Expected", domain.ErrPersonNotFound, "got", e)
	}
}

func TestPersonServiceList_GreaterWindow(t *testing.T) {
	data := []*domain.Person{
		{ID: uuid.MustParse("019ec1a9-0f6f-7d39-9dd4-3d98e722f36c"), FirstName: "Дмитрий", LastName: "Верещагин", Age: 38, Gender: 1, Nationality: "RU", Emails: []string{"dima@mail.ru"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3a-9042-029490c95bb8"), FirstName: "Ольга", LastName: "Назарова", Age: 22, Gender: 2, Nationality: "RU", Emails: []string{"olga.work@company.com", "nazarova.o@yandex.ru"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3b-ac25-91e9534bea6a"), FirstName: "Jane", LastName: "Doe", Age: 31, Gender: 2, Nationality: "US", Emails: []string{"janedoe@gmail.com"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3c-b826-2b6b266bc642"), FirstName: "Juan", LastName: "Xavier", Age: 57, Gender: 1, Nationality: "BR", Emails: []string{}},
	}

	repo := &mockRepo{
		listFunc: func(ctx context.Context, offset, limit uint32) ([]*domain.Person, uint32, error) {
			total := uint32(len(data))

			if offset >= total {
				return []*domain.Person{}, total, nil
			}

			end := offset + limit
			if end > total {
				end = total
			}

			result := data[offset:end]

			return result, total, nil
		},
	}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}

	useCase := usecase.NewPersonUseCase(repo, enricher)
	persons, total, page, limit, _ := useCase.ListPersons(context.Background(), 0, 100000)

	if page != 1 {
		t.Error("Expected page switched to 1, got", page)
	}
	if limit != 100 {
		t.Error("Expected limit switched to 100, got", limit)
	}
	if total != uint32(len(data)) {
		t.Errorf("Expected total count eq %d, got %d", len(data), total)
	}
	if len(persons) != len(data) {
		t.Errorf("Expected len(persons)=%d, got=%d", len(data), len(persons))
	}
}

func TestPersonServiceList_GreaterPage(t *testing.T) {
	data := []*domain.Person{
		{ID: uuid.MustParse("019ec1a9-0f6f-7d39-9dd4-3d98e722f36c"), FirstName: "Дмитрий", LastName: "Верещагин", Age: 38, Gender: 1, Nationality: "RU", Emails: []string{"dima@mail.ru"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3a-9042-029490c95bb8"), FirstName: "Ольга", LastName: "Назарова", Age: 22, Gender: 2, Nationality: "RU", Emails: []string{"olga.work@company.com", "nazarova.o@yandex.ru"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3b-ac25-91e9534bea6a"), FirstName: "Jane", LastName: "Doe", Age: 31, Gender: 2, Nationality: "US", Emails: []string{"janedoe@gmail.com"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3c-b826-2b6b266bc642"), FirstName: "Juan", LastName: "Xavier", Age: 57, Gender: 1, Nationality: "BR", Emails: []string{}},
	}

	repo := &mockRepo{
		listFunc: func(ctx context.Context, offset, limit uint32) ([]*domain.Person, uint32, error) {
			total := uint32(len(data))

			if offset >= total {
				return []*domain.Person{}, total, nil
			}

			end := offset + limit
			if end > total {
				end = total
			}

			result := data[offset:end]

			return result, total, nil
		},
	}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}

	useCase := usecase.NewPersonUseCase(repo, enricher)
	persons, total, page, limit, _ := useCase.ListPersons(context.Background(), 3, 5)

	if page != 3 {
		t.Error("Expected page to be eq 3, got", page)
	}
	if limit != 5 {
		t.Error("Expected limit to be eq 5, got", limit)
	}
	if total != uint32(len(data)) {
		t.Errorf("Expected total count eq %d, got %d", len(data), total)
	}
	if len(persons) != 0 {
		t.Error("Expected no persons, got", len(persons))
	}
}

func TestPersonServiceList_PartialOverlap(t *testing.T) {
	data := []*domain.Person{
		{ID: uuid.MustParse("019ec1a9-0f6f-7d39-9dd4-3d98e722f36c"), FirstName: "Дмитрий", LastName: "Верещагин", Age: 38, Gender: 1, Nationality: "RU", Emails: []string{"dima@mail.ru"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3a-9042-029490c95bb8"), FirstName: "Ольга", LastName: "Назарова", Age: 22, Gender: 2, Nationality: "RU", Emails: []string{"olga.work@company.com", "nazarova.o@yandex.ru"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3b-ac25-91e9534bea6a"), FirstName: "Jane", LastName: "Doe", Age: 31, Gender: 2, Nationality: "US", Emails: []string{"janedoe@gmail.com"}},
		{ID: uuid.MustParse("019ec1a9-0f6f-7d3c-b826-2b6b266bc642"), FirstName: "Juan", LastName: "Xavier", Age: 57, Gender: 1, Nationality: "BR", Emails: []string{}},
	}

	repo := &mockRepo{
		listFunc: func(ctx context.Context, offset, limit uint32) ([]*domain.Person, uint32, error) {
			total := uint32(len(data))

			if offset >= total {
				return []*domain.Person{}, total, nil
			}

			end := offset + limit
			if end > total {
				end = total
			}

			result := data[offset:end]

			return result, total, nil
		},
	}
	enricher := &mockEnricher{enrichFunc: func(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
		return 0, domain.GenderUnspecified, "", nil
	}}

	useCase := usecase.NewPersonUseCase(repo, enricher)
	persons, total, _, _, _ := useCase.ListPersons(context.Background(), 2, 3)

	if total != uint32(len(data)) {
		t.Errorf("Expected total count eq %d, got %d", len(data), total)
	}
	if len(persons) != 1 {
		t.Error("Expected one single person, got", len(persons))
	}
}

func TestPersonServiceUpdate_PatchAgeOnly(t *testing.T) {
	targetID, _ := uuid.NewV7()
	repo := &mockRepo{
		getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
			return &domain.Person{
				ID:        targetID,
				FirstName: "Hans",
				Age:       40,
				Emails:    []string{"old@mail.com"},
			}, nil
		},
		updateFunc: func(ctx context.Context, person *domain.Person) error {
			if person.Age != 45 {
				t.Errorf("Expected age to be updated to 45, got %d", person.Age)
			}
			if person.FirstName != "Hans" {
				t.Errorf("Expected FirstName to remain Hans, got %s", person.FirstName)
			}
			if len(person.Emails) != 1 {
				t.Errorf("Expected emails count to remain 1, got %d", len(person.Emails))
			}
			if person.Emails[0] != "old@mail.com" {
				t.Errorf("Expected Emails to remain old, got %s", person.Emails[0])
			}
			return nil
		},
	}

	useCase := usecase.NewPersonUseCase(repo, nil)

	newAge := uint32(45)
	_, err := useCase.UpdatePerson(context.Background(), targetID, ports.UpdatePersonInput{
		Age: &newAge,
	})

	if err != nil {
		t.Fatal("Unexpected error on update", err)
	}
}

func TestPersonServiceUpdate_AddNewEmail(t *testing.T) {
	targetID, _ := uuid.NewV7()
	repo := &mockRepo{
		getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
			return &domain.Person{
				ID:        targetID,
				FirstName: "Hans",
				Age:       40,
				Emails:    []string{"old@mail.com"},
			}, nil
		},
		updateFunc: func(ctx context.Context, person *domain.Person) error {
			if len(person.Emails) != 2 {
				t.Errorf("Expected emails count to become 2, got %d", len(person.Emails))
			}
			if person.Emails[1] != "new@mail.com" {
				t.Errorf("Expected Emails new email to become [1], got %s", person.Emails[0])
			}
			return nil
		},
	}

	useCase := usecase.NewPersonUseCase(repo, nil)

	_, err := useCase.UpdatePerson(context.Background(), targetID, ports.UpdatePersonInput{
		Emails: &[]string{"old@mail.com", "new@mail.com"},
	})

	if err != nil {
		t.Fatal("Unexpected error on update", err)
	}
}

func TestPersonServiceUpdate_DeleteExistingEmail(t *testing.T) {
	targetID, _ := uuid.NewV7()
	repo := &mockRepo{
		getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
			return &domain.Person{
				ID:        targetID,
				FirstName: "Hans",
				Age:       40,
				Emails:    []string{"old@mail.com"},
			}, nil
		},
		updateFunc: func(ctx context.Context, person *domain.Person) error {
			if len(person.Emails) != 0 {
				t.Errorf("Expected emails count to become 0, got %d", len(person.Emails))
			}
			return nil
		},
	}

	useCase := usecase.NewPersonUseCase(repo, nil)

	_, err := useCase.UpdatePerson(context.Background(), targetID, ports.UpdatePersonInput{
		Emails: &[]string{},
	})

	if err != nil {
		t.Fatal("Unexpected error on update", err)
	}
}

func TestPersonServiceUpdate_RewriteEmails(t *testing.T) {
	targetID, _ := uuid.NewV7()
	repo := &mockRepo{
		getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
			return &domain.Person{
				ID:        targetID,
				FirstName: "Hans",
				Age:       40,
				Emails:    []string{"old1@mail.com", "old2@mail.com"},
			}, nil
		},
		updateFunc: func(ctx context.Context, person *domain.Person) error {
			if len(person.Emails) != 2 {
				t.Errorf("Expected emails count to remain 2, got %d", len(person.Emails))
			}
			if person.Emails[0] != "new1@mail.com" {
				t.Errorf("Expected Emails[0] to become new1@..., got %s", person.Emails[0])
			}
			if person.Emails[1] != "new2@mail.com" {
				t.Errorf("Expected Emails[1] to become new2@..., got %s", person.Emails[1])
			}
			return nil
		},
	}

	useCase := usecase.NewPersonUseCase(repo, nil)

	_, err := useCase.UpdatePerson(context.Background(), targetID, ports.UpdatePersonInput{
		Emails: &[]string{"new1@mail.com", "new2@mail.com"},
	})

	if err != nil {
		t.Fatal("Unexpected error on update", err)
	}
}

func TestPersonServiceUpdate_UpsertInvalidEmails(t *testing.T) {
	targetID, _ := uuid.NewV7()
	repo := &mockRepo{
		getByIdFunc: func(ctx context.Context, id uuid.UUID) (*domain.Person, error) {
			return &domain.Person{
				ID:        targetID,
				FirstName: "Hans",
				Age:       40,
				Emails:    []string{"old1@mail.com", "old2@mail.com"},
			}, nil
		},
		updateFunc: func(ctx context.Context, person *domain.Person) error {
			if len(person.Emails) != 2 {
				t.Errorf("Expected emails count to remain 2, got %d", len(person.Emails))
			}
			if person.Emails[0] != "old1@mail.com" {
				t.Errorf("Expected Emails[0] to remain old1@..., got %s", person.Emails[0])
			}
			if person.Emails[1] != "old2@mail.com" {
				t.Errorf("Expected Emails[1] to remain old2@..., got %s", person.Emails[1])
			}
			return nil
		},
	}

	useCase := usecase.NewPersonUseCase(repo, nil)

	_, err := useCase.UpdatePerson(context.Background(), targetID, ports.UpdatePersonInput{
		Emails: &[]string{"old1@mail.com", "old2@mail.com", "email-invalid@test", "email-invalid", "@mail.ru"},
	})

	if err != nil {
		t.Fatal("Unexpected error on update", err)
	}
}

func TestPersonServiceDelete_Success(t *testing.T) {
	targetID, _ := uuid.NewV7()
	wasCalled := false

	repo := &mockRepo{
		deleteFunc: func(ctx context.Context, id uuid.UUID) error {
			wasCalled = true
			if id != targetID {
				t.Errorf("Expected delete to be called with ID %s, got %s", targetID, id)
			}
			return nil
		},
	}

	useCase := usecase.NewPersonUseCase(repo, nil)
	err := useCase.DeletePerson(context.Background(), targetID)

	if err != nil {
		t.Fatal("Unexpected error on delete", err)
	}
	if !wasCalled {
		t.Error("Repository Delete method was never called")
	}
}

func TestPersonServiceDelete_InvalidUUID(t *testing.T) {
	repo := &mockRepo{}
	useCase := usecase.NewPersonUseCase(repo, nil)

	err := useCase.DeletePerson(context.Background(), uuid.Nil)

	if err == nil {
		t.Error("Expected error for uuid.Nil, got none")
	}
	if !errors.Is(err, domain.ErrInvalidData) {
		t.Errorf("Expected error to wrap %v, got %v", domain.ErrInvalidData, err)
	}
}

func TestPersonServiceDelete_PersonNotFound(t *testing.T) {
	targetID, _ := uuid.NewV7()

	repo := &mockRepo{
		deleteFunc: func(ctx context.Context, id uuid.UUID) error {
			return domain.ErrPersonNotFound
		},
	}
	useCase := usecase.NewPersonUseCase(repo, nil)

	err := useCase.DeletePerson(context.Background(), targetID)

	if err == nil {
		t.Error("Expected error when person does not exist, got none")
	}
	if !errors.Is(err, domain.ErrPersonNotFound) {
		t.Errorf("Expected error to wrap %v, got %v", domain.ErrPersonNotFound, err)
	}
}
