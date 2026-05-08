package auth

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(user uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		Subject:   user.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn)),
	})
	tokenString, err := newToken.SignedString([]byte(tokenSecret))
	if err != nil {
		log.Printf("issue encrypting key: %v", err)
		return tokenString, err
	}

	return tokenString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		log.Printf("Failed to ParseWithClaims: %v", err)
		return uuid.Nil, err
	}
	extractedUUID, err := claims.GetSubject()
	if err != nil {
		log.Printf("failed to get uuid: %v", err)
		return uuid.Nil, err
	}
	parsedUUID, err := uuid.Parse(extractedUUID)
	if err != nil {
		log.Printf("failed to parse UUID: %v", err)
		return uuid.Nil, err
	}
	return parsedUUID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	header := headers.Get("Authorization")
	if header == "" {
		return "", fmt.Errorf("no Authorization header provided")
	}
	tokenSlice := strings.Split(header, " ")
	if tokenSlice[0] != "Bearer" {
		return "", fmt.Errorf("not a bearer token")
	}
	return tokenSlice[1], nil
}
