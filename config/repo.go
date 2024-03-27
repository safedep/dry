package config

import (
	"fmt"
	"os"
)

// A base repository to hold common code, keeping things DRY
// at repository level
type baseRepository[T any] struct{}

func (b *baseRepository[T]) Default(c *Config[T]) (T, error) {
	if c.MustSupply {
		return c.Default, fmt.Errorf("config: %s is required", c.Name)
	}

	return c.Default, nil
}

type EnvironmentRepositoryConfig[T any] struct {
	Prefix  string
	Encoder ConfigEncoder[T]
}

type environmentRepository[T any] struct {
	baseRepository[T]
	config EnvironmentRepositoryConfig[T]
}

func NewEnvironmentRepository[T any](config EnvironmentRepositoryConfig[T]) (ConfigRepository[T], error) {
	if config.Encoder == nil {
		config.Encoder = &JSONConfigEncoder[T]{}
	}

	return &environmentRepository[T]{config: config}, nil
}

func (e *environmentRepository[T]) GetConfig(c *Config[T]) (T, error) {
	key := fmt.Sprintf("%s%s", e.config.Prefix, c.Name)
	value := os.Getenv(key)

	if value == "" {
		return e.Default(c)
	}

	return e.config.Encoder.Decode(value)
}
