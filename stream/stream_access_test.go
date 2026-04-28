package stream

import (
	"errors"
	"testing"
	"time"
)

func TestNormalizeExpiry(t *testing.T) {
	tests := []struct {
		name    string
		input   time.Duration
		want    time.Duration
		wantErr error
	}{
		{"zero applies default", 0, StreamAccessDefaultExpiry, nil},
		{"below min rejected", 30 * time.Second, 0, ErrExpiryTooShort},
		{"exactly min accepted", StreamAccessMinExpiry, StreamAccessMinExpiry, nil},
		{"in range passes through", 2 * time.Hour, 2 * time.Hour, nil},
		{"exactly max accepted", StreamAccessMaxExpiry, StreamAccessMaxExpiry, nil},
		{"above max rejected", 48 * time.Hour, 0, ErrExpiryTooLong},
		{"negative rejected", -1 * time.Second, 0, ErrExpiryTooShort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeExpiry(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want err %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %v, got %v", tt.want, got)
			}
		})
	}
}

func TestStreamScopeValidate(t *testing.T) {
	tests := []struct {
		name    string
		scope   StreamScope
		wantErr error
	}{
		{"tenant only", StreamScope{TenantID: "t1"}, nil},
		{"namespace only", StreamScope{Namespace: "ns1"}, nil},
		{"name-prefix only", StreamScope{NamePrefix: "foo"}, nil},
		{"tenant + namespace", StreamScope{TenantID: "t1", Namespace: "ns1"}, nil},
		{"all three set", StreamScope{TenantID: "t1", Namespace: "ns1", NamePrefix: "foo"}, nil},
		{"all empty rejected", StreamScope{}, ErrInvalidScope},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.scope.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want err %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		})
	}
}

func TestStreamAccessRequestValidate(t *testing.T) {
	validStream := Stream{Namespace: "ns1", Name: "s1"}
	validScope := &StreamScope{TenantID: "t1"}

	tests := []struct {
		name    string
		req     StreamAccessRequest
		wantErr error
	}{
		{
			name:    "valid single-stream read",
			req:     StreamAccessRequest{Stream: validStream, Access: StreamAccessRead, Expiry: time.Hour},
			wantErr: nil,
		},
		{
			name:    "valid scoped readwrite",
			req:     StreamAccessRequest{Scope: validScope, Access: StreamAccessReadWrite, Expiry: time.Hour},
			wantErr: nil,
		},
		{
			name:    "access none rejected",
			req:     StreamAccessRequest{Stream: validStream, Access: StreamAccessNone, Expiry: time.Hour},
			wantErr: ErrInvalidAccessRole,
		},
		{
			name:    "access out of range rejected",
			req:     StreamAccessRequest{Stream: validStream, Access: StreamAccessRole(99), Expiry: time.Hour},
			wantErr: ErrInvalidAccessRole,
		},
		{
			name:    "expiry too short",
			req:     StreamAccessRequest{Stream: validStream, Access: StreamAccessRead, Expiry: time.Second},
			wantErr: ErrExpiryTooShort,
		},
		{
			name:    "expiry too long",
			req:     StreamAccessRequest{Stream: validStream, Access: StreamAccessRead, Expiry: 48 * time.Hour},
			wantErr: ErrExpiryTooLong,
		},
		{
			name:    "zero expiry accepted (default applied elsewhere)",
			req:     StreamAccessRequest{Stream: validStream, Access: StreamAccessRead, Expiry: 0},
			wantErr: nil,
		},
		{
			name:    "empty scope rejected",
			req:     StreamAccessRequest{Scope: &StreamScope{}, Access: StreamAccessRead, Expiry: time.Hour},
			wantErr: ErrInvalidScope,
		},
		{
			name:    "nil scope with invalid stream rejected",
			req:     StreamAccessRequest{Stream: Stream{}, Access: StreamAccessRead, Expiry: time.Hour},
			wantErr: nil, // Stream.ID() error wrapped; asserted below by fallthrough check
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.name == "nil scope with invalid stream rejected" {
				if err == nil {
					t.Fatalf("expected error for empty stream")
				}
				return
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want err %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		})
	}
}
