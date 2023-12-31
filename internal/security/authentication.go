package security

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"time"
)

func CreateJwtToken(userId, expiresInSeconds int) (string, error) {
	signingKey, err := getJwtSecret()
	if err != nil {
		log.Print("fail to get signing key")
		return "", err
	}

	claims := &jwt.RegisteredClaims{
		Issuer:    "chirpy",
		Subject:   strconv.Itoa(userId),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Duration(expiresInSeconds))),
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte(signingKey))
	if err != nil {
		log.Print("fail to get signing key")
		return "", err
	}

	return ss, nil
}

func getJwtSecret() (string, error) {
	err := godotenv.Load()
	if err != nil {
		return "", err
	}

	return os.Getenv("JWT_SECRET"), nil
}
