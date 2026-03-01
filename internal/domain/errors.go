package domain

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrSubnetNotFound = errors.New("subnet not found")
	ErrIPNotFound     = errors.New("ip not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrConflict       = errors.New("conflict")
	ErrUnauthorized   = errors.New("unauthorized")
)
