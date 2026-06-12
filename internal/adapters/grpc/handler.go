package grpc

import (
	"context"
	"errors"
	"person-grpc/internal/domain"
	"person-grpc/internal/ports"
	"person-grpc/pkg/personpb"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PersonHandler struct {
	personpb.UnimplementedPersonServiceServer
	useCase ports.PersonUseCase
}

func NewPersonHandler(useCase ports.PersonUseCase) *PersonHandler {
	return &PersonHandler{useCase: useCase}
}

/*
Util section
*/

func mapPersonToProtobuf(person *domain.Person) *personpb.Person {
	if person == nil {
		return nil
	}
	return &personpb.Person{
		Id:          person.ID.String(),
		FirstName:   person.FirstName,
		LastName:    person.LastName,
		Patronymic:  person.Patronymic,
		Age:         person.Age,
		Gender:      personpb.Gender(person.Gender),
		Nationality: person.Nationality,
		Emails:      person.Emails,
	}
}

/*
Main section
*/

func (h *PersonHandler) CreatePerson(ctx context.Context, req *personpb.CreatePersonRequest) (*personpb.Person, error) {
	var patronymic *string
	if req != nil {
		patronymic = req.Patronymic
	}
	person, err := h.useCase.CreatePerson(ctx, req.GetFirstName(), req.GetLastName(), patronymic, req.GetEmails())
	if err != nil {
		if errors.Is(err, domain.ErrInvalidData) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to create person: %v", err)
	}

	return mapPersonToProtobuf(person), nil
}

func (h *PersonHandler) GetPerson(ctx context.Context, req *personpb.GetPersonRequest) (*personpb.Person, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid uuid format: %v", err.Error())
	}

	person, err := h.useCase.GetPerson(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPersonNotFound) {
			return nil, status.Error(codes.NotFound, "person not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get person: %v", err)
	}

	return mapPersonToProtobuf(person), nil
}

func (h *PersonHandler) ListPersons(ctx context.Context, req *personpb.ListPersonsRequest) (*personpb.ListPersonsResponse, error) {
	persons, total, page, limit, err := h.useCase.ListPersons(ctx, req.GetPage(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list persons: %v", err)
	}

	pbPersons := make([]*personpb.Person, len(persons))
	for i, person := range persons {
		pbPersons[i] = mapPersonToProtobuf(person)
	}

	return &personpb.ListPersonsResponse{
		Data:  pbPersons,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (h *PersonHandler) UpdatePerson(ctx context.Context, req *personpb.UpdatePersonRequest) (*personpb.Person, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid uuid format: %v", err.Error())
	}

	input := ports.UpdatePersonInput{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Patronymic:  req.Patronymic,
		Age:         req.Age,
		Nationality: req.Nationality,
	}
	if req.Gender != nil {
		g := domain.Gender(*req.Gender)
		input.Gender = &g
	}
	if req.Emails != nil {
		emails := req.GetEmails().GetValues()
		input.Emails = &emails
	}

	person, err := h.useCase.UpdatePerson(ctx, id, input)
	if err != nil {
		if errors.Is(err, domain.ErrPersonNotFound) {
			return nil, status.Error(codes.NotFound, "person not found")
		}
		if errors.Is(err, domain.ErrInvalidData) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to update person: %v", err)
	}

	return mapPersonToProtobuf(person), nil
}

func (h *PersonHandler) DeletePerson(ctx context.Context, req *personpb.DeletePersonRequest) (*personpb.DeletePersonResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid uuid format: %v", err.Error())
	}

	err = h.useCase.DeletePerson(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPersonNotFound) {
			return nil, status.Error(codes.NotFound, "person not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to delete person: %v", err)
	}

	return &personpb.DeletePersonResponse{}, nil
}
