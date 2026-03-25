package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name           string
		signSecret     string
		validateSecret string
		expiresIn      time.Duration
		wantErr        bool
	}{
		{
			name:           "valid token",
			signSecret:     "mysecret",
			validateSecret: "mysecret",
			expiresIn:      time.Hour,
			wantErr:        false,
		},
		{
			name:           "expired token",
			signSecret:     "mysecret",
			validateSecret: "mysecret",
			expiresIn:      -time.Second,
			wantErr:        true,
		},
		{
			name:           "wrong secret",
			signSecret:     "mysecret",
			validateSecret: "wrongsecret",
			expiresIn:      time.Hour,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := MakeJWT(userID, tt.signSecret, tt.expiresIn)
			if err != nil {
				t.Fatalf("MakeJWT() error = %v", err)
			}
			_, err = ValidateJWT(token, tt.validateSecret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJWT() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	tests := []struct {
		name          string
		headerValue   string
		expectedToken string
		expectError   bool
	}{
		{
			name:          "valid bearer token",
			headerValue:   "Bearer abc123",
			expectedToken: "abc123",
			expectError:   false,
		},
		{
			name:        "missing authorization header",
			headerValue: "",
			expectError: true,
		},
		{
			name:        "wrong authorization scheme",
			headerValue: "Basic abc123",
			expectError: true,
		},
		{
			name:        "bearer with no token",
			headerValue: "Bearer ",
			expectError: true,
		},
		{
			name:          "bearer token with extra spaces",
			headerValue:   "Bearer    xyz789   ",
			expectedToken: "xyz789",
			expectError:   false,
		},
		{
			name:        "bearer keyword only",
			headerValue: "Bearer",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.headerValue != "" {
				headers.Set("Authorization", tt.headerValue)
			}

			token, err := GetBearerToken(headers)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if token != tt.expectedToken {
				t.Fatalf("expected token %q, got %q", tt.expectedToken, token)
			}
		})
	}
}
