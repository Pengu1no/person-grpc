package usecase

import (
	"net/mail"
	"person-grpc/internal/domain"
	"strings"
)

func validateEmail(email string) error {
	addr, e := mail.ParseAddress(email)
	if e != nil {
		return domain.ErrInvalidEmail
	}

	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return domain.ErrInvalidEmail
	}
	domainPt := parts[1]

	lastDot := strings.LastIndex(domainPt, ".")
	if lastDot == -1 {
		return domain.ErrInvalidEmail
	}

	if lastDot > len(domainPt)-3 {
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
