package stream

import "testing"

func TestStreamID(t *testing.T) {
	tests := []struct {
		name        string
		stream      Stream
		expectedID  string
		expectError bool
	}{
		{
			name: "Global stream",
			stream: Stream{
				Namespace: "global",
				Name:      "test-stream",
			},
			expectedID: "global:test-stream",
		},
		{
			name: "Multi-tenant stream with TenantID",
			stream: Stream{
				TenantID:      "tenant1",
				Namespace:     "namespace1",
				Name:          "test-stream",
				IsMultiTenant: true,
			},
			expectedID: "tenant1:namespace1:test-stream",
		},
		{
			name: "Multi-tenant stream without TenantID",
			stream: Stream{
				IsMultiTenant: true,
				Namespace:     "namespace1",
				Name:          "test-stream",
			},
			expectError: true,
		},
		{
			name: "Stream with resource ID",
			stream: Stream{
				Namespace:  "namespace1",
				Name:       "test-stream",
				resourceID: "resource123",
			},
			expectedID: "namespace1:test-stream:resource123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.stream.ID()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if id != tt.expectedID {
				t.Errorf("expected ID %s, got %s", tt.expectedID, id)
			}
		})
	}
}
