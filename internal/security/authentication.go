package security

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"os"
	"strconv"
	"time"
)

func CreateJwtToken(userId, expiresInSeconds int) (string, error) {
	signingKey := getJwtSecret()
	nowUTC := time.Now().UTC()

	claims := &jwt.RegisteredClaims{
		Issuer:    "chirpy",
		Subject:   strconv.Itoa(userId),
		ExpiresAt: jwt.NewNumericDate(nowUTC.Add(time.Second * time.Duration(expiresInSeconds))),
		IssuedAt:  jwt.NewNumericDate(nowUTC),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte(signingKey))
	if err != nil {
		log.Print("fail to get signing key")
		return "", err
	}

	return ss, nil
}

func GetTokenClaims(tokenString string) (jwt.Claims, error) {
	// Validate Token
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(getJwtSecret()), nil
	})

	if err != nil {
		log.Print(err)
		return jwt.RegisteredClaims{}, err
	}

	if !token.Valid {
		log.Print("invalid token")
		return jwt.RegisteredClaims{}, errors.New("token is invalid")
	}

	return token.Claims, nil
}

func getJwtSecret() string {
	return os.Getenv("JWT_SECRET")
}
