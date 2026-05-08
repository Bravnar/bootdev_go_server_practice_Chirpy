package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "my-test-secret"
	token, err := MakeJWT(userID, secret, time.Minute*2)
	if err != nil {
		t.Errorf("MakeJWT function returned an error")
	}
	returnedUUID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Errorf("ValidateJWT returned an error")
	}
	if userID != returnedUUID {
		t.Fatalf("the UUIDs are not the same, expected: %v, got %v", userID, returnedUUID)
	}
}

func TestValidateJWTWrongSecret(t *testing.T) {
	userID := uuid.New()
	secret := "my-test-secret"
	token, err := MakeJWT(userID, secret, time.Minute*2)
	if err != nil {
		t.Errorf("MakeJWT function returned an error")
	}
	_, err = ValidateJWT(token, "another-test-secret")
	if err == nil {
		t.Errorf("ValidateJWT function did not return an error")
	}
}
