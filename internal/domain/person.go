package domain

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrPersonNotFound = errors.New("person not found")
	ErrInvalidData    = errors.New("invalid person data")
)

type Gender int32

const (
	GenderUnspecified Gender = 0
	GenderMale        Gender = 1
	GenderFemale      Gender = 2
)

type Person struct {
	ID          uuid.UUID
	FirstName   string
	LastName    string
	Patronymic  *string
	Age         uint32
	Gender      Gender
	Nationality string
	Emails      []string
}
