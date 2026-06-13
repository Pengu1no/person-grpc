package domain

import "errors"

var (
	ErrPersonNotFound = errors.New("person not found")
	ErrInvalidData    = errors.New("invalid person data")
	ErrInvalidEmail   = errors.New("invalid email format")
	ErrInvalidAge     = errors.New("age must be between 1 and 120")
)
