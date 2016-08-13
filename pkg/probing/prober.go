package probing

import (
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("probing: id not found")
	ErrExist    = errors.New("probing: id already exists")
)

// Prober defines probing operation.
type Prober interface {
	AddHTTP(id string, interval time.Duration, endpoints []string) error

	Remove(id string) error
	RemoveAll()

	Reset(id string) error

	Status(id string) (Status, error)
}
