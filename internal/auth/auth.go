// Package auth provides a simmple implementation for hashing
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", nil
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	ok, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		log.Printf("ComparePasswordAndHash function failed: %v", err)
		return false, err
	}
	if ok {
		return true, nil
	}
	return false, nil
}

func MakeRefreshToken() string {
	refreshToken := make([]byte, 32)
	rand.Read(refreshToken)
	return hex.EncodeToString(refreshToken)
}

func GetAPIToken(headers http.Header) (string, error) {
	header := headers.Get("Authorization")
	if header == "" {
		return "", fmt.Errorf("not Authorization header provided")
	}
	tokenSlice := strings.Split(header, " ")
	if tokenSlice[0] != "ApiKey" {
		return "", fmt.Errorf("not an api-key")
	}
	return tokenSlice[1], nil
}
