package utils

import (
	"errors"
	"reflect"

	"github.com/go-playground/validator/v10"
)

var (
	// Initialize our opinionated configuration of the validator
	// Any change here must be strongly tested for downstream impact
	validate *validator.Validate = validator.New(validator.WithRequiredStructEnabled())

	ErrValidatorNilValue = errors.New("validator cannot validate nil value")
)

func init() {
	// Placeholder for registering custom mapping and rules in the validator
	// We do this so that we can enforce our opinionated mappings across all downstream consumers
}

// ValidateStruct validates a Go struct using https://github.com/go-playground/validator v10
// Use this to validate config, request, response and other data types that are represented as Go
// structs instead of manually checking using if/else.
//
// Example:
//
//	type SomeConfig {
//			MyNotEmptyvalue string `validate:"required"`
//			MyEmail string `validate:"required,email"`
//	}
//
//	if err := utils.ValidateStruct(someConfig); err != nil {
//		// Handle error
//	}
func ValidateStruct(st any) error {
	// Handle fail fast checks
	val := reflect.ValueOf(st)
	if val.Kind() == reflect.Pointer && val.IsNil() {
		return ErrValidatorNilValue
	}

	return validate.Struct(st)
}
