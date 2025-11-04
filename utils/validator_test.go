package utils

import (
	"testing"
)

func TestValidateStruct(t *testing.T) {
	testCases := []struct {
		name    string
		build   func() any
		wantErr bool
	}{
		{
			name: "valid struct",
			build: func() any {
				type input struct {
					RequiredField string `validate:"required"`
					EmailField    string `validate:"omitempty,email"`
				}
				return input{
					RequiredField: "value",
					EmailField:    "test@example.com",
				}
			},
			wantErr: false,
		},
		{
			name: "valid struct with optional field empty",
			build: func() any {
				type input struct {
					RequiredField string `validate:"required"`
					EmailField    string `validate:"omitempty,email"`
				}
				return input{
					RequiredField: "value",
				}
			},
			wantErr: false,
		},
		{
			name: "invalid struct with required field empty",
			build: func() any {
				type input struct {
					RequiredField string `validate:"required"`
					EmailField    string `validate:"omitempty,email"`
				}
				return input{
					EmailField: "test@example.com",
				}
			},
			wantErr: true,
		},
		{
			name: "invalid struct with invalid email",
			build: func() any {
				type input struct {
					RequiredField string `validate:"required"`
					EmailField    string `validate:"omitempty,email"`
				}
				return input{
					RequiredField: "value",
					EmailField:    "not-an-email",
				}
			},
			wantErr: true,
		},
		{
			name:    "nil value",
			build: func() any {
				return nil
			},
			wantErr: true,
		},
		{
			name:    "nil pointer to struct",
			build: func() any {
				type input struct {
					RequiredField string `validate:"required"`
				}
				var ptr *input
				return ptr
			},
			wantErr: true,
		},
		{
			name: "valid struct with min length",
			build: func() any {
				type input struct {
					Password string `validate:"required,min=8"`
				}
				return input{
					Password: "supersecret",
				}
			},
			wantErr: false,
		},
		{
			name: "invalid struct with min length",
			build: func() any {
				type input struct {
					Password string `validate:"required,min=8"`
				}
				return input{
					Password: "short",
				}
			},
			wantErr: true,
		},
		{
			name: "valid struct with numeric range",
			build: func() any {
				type input struct {
					Count int `validate:"gte=1,lte=10"`
				}
				return input{
					Count: 5,
				}
			},
			wantErr: false,
		},
		{
			name: "invalid struct with numeric range",
			build: func() any {
				type input struct {
					Count int `validate:"gte=1,lte=10"`
				}
				return input{
					Count: 0,
				}
			},
			wantErr: true,
		},
		{
			name: "valid struct with slice dive rule",
			build: func() any {
				type input struct {
					Tags []string `validate:"required,dive,alpha"`
				}
				return input{
					Tags: []string{"stable", "beta"},
				}
			},
			wantErr: false,
		},
		{
			name: "invalid struct with slice dive rule",
			build: func() any {
				type input struct {
					Tags []string `validate:"required,dive,alpha"`
				}
				return input{
					Tags: []string{"valid", "inv@lid"},
				}
			},
			wantErr: true,
		},
		{
			name: "invalid struct with required nested pointer",
			build: func() any {
				type nested struct {
					Value string `validate:"required"`
				}
				type input struct {
					Nested *nested `validate:"required"`
				}
				return input{
					Nested: nil,
				}
			},
			wantErr: true,
		},
		{
			name: "valid struct with required nested pointer",
			build: func() any {
				type nested struct {
					Value string `validate:"required"`
				}
				type input struct {
					Nested *nested `validate:"required"`
				}
				return input{
					Nested: &nested{Value: "ok"},
				}
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(tc.build())
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateStruct() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
