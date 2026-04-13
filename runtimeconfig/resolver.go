package runtimeconfig

import (
	"context"
	"errors"
	"net/http"
)

var (
	ErrNilSource = errors.New("runtimeconfig: source is nil")
	ErrNilBinder = errors.New("runtimeconfig: binder is nil")
	ErrNilValue  = errors.New("runtimeconfig: binder returned nil value")
)

type Resolver[T any] interface {
	Resolve(context.Context, *http.Request) (*T, error)
}

type Binder[C any, T any] func(*C) (*T, error)

type Mutator[T any] func(context.Context, *http.Request, *T) error

type Option[T any] func(*settings[T])

type settings[T any] struct {
	mutator Mutator[T]
}

type resolver[C any, T any] struct {
	source *C
	bind   Binder[C, T]
	config settings[T]
}

func NewResolver[C any, T any](source *C, bind Binder[C, T], opts ...Option[T]) Resolver[T] {
	cfg := settings[T]{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return &resolver[C, T]{
		source: source,
		bind:   bind,
		config: cfg,
	}
}

func WithMutator[T any](mutator Mutator[T]) Option[T] {
	return func(cfg *settings[T]) {
		cfg.mutator = mutator
	}
}

func (r *resolver[C, T]) Resolve(ctx context.Context, req *http.Request) (*T, error) {
	if r.source == nil {
		return nil, ErrNilSource
	}
	if r.bind == nil {
		return nil, ErrNilBinder
	}

	value, err := r.bind(r.source)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, ErrNilValue
	}

	if r.config.mutator != nil {
		if err := r.config.mutator(ctx, req, value); err != nil {
			return nil, err
		}
	}

	return value, nil
}
