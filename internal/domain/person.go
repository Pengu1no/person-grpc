package domain

import (
	"github.com/google/uuid"
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
