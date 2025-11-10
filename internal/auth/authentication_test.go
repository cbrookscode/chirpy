package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateAndValidateJWT(t *testing.T) {
	type testToken struct {
		id          uuid.UUID
		tokenSecret string
		duration    time.Duration
	}

	tests := []testToken{
		{uuid.New(), "dinosaurs", time.Minute},
		{uuid.New(), "computers", time.Minute},
		{uuid.New(), "wiley", time.Minute},
		{uuid.New(), "booger", time.Minute},
		{uuid.New(), "puke", time.Minute},
	}

	for _, tt := range tests {
		tt := tt // capture variable at current loop iteration
		t.Run(tt.tokenSecret, func(t *testing.T) {
			t.Parallel() // run each subtest concurrently
			tokenString, err := MakeJWT(tt.id, tt.tokenSecret, tt.duration)
			if err != nil {
				t.Errorf("error making jwt: %v", err)
			}
			tokenUUID, err := ValidateJWT(tokenString, tt.tokenSecret)
			if err != nil {
				t.Errorf("error validating jwt: %v", err)
			}
			if tokenUUID != tt.id {
				t.Errorf("ids did not match: got %v, want %v", tokenUUID, tt.id)
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	testHeader := http.Header{
		"Authorization": []string{"Bearer tokenString"},
	}

	tokenstring, err := GetBearerToken(testHeader)
	if err != nil {
		t.Errorf("error getting bearer token: %v", err)
	}
	if tokenstring != "tokenString" {
		t.Errorf("string doesn't match expectation. Got %v, Want tokenString", tokenstring)
	}
}
