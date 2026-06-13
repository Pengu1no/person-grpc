package usecase

import (
	"net/mail"
	"person-grpc/internal/domain"
)

func validateEmail(email string) error {
	_, e := mail.ParseAddress(email)
	if e != nil {
		return domain.ErrInvalidEmail
	}
	return nil
}

func validateAge(age uint32) error {
	if age < 1 || age > 120 {
		return domain.ErrInvalidAge
	}
	return nil
}
