package domain

import "errors"

var (
	ErrInvalidInput          = errors.New("notification: invalid input")
	ErrNotFoundOrAlreadyRead = errors.New("notification: not found or already read")
)
