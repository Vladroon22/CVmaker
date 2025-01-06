package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/Vladroon22/CVmaker/internal/ut"
	"github.com/dgrijalva/jwt-go"
)

type JwtClaims struct {
	jwt.StandardClaims
	UserID int
}

func GenerateJWT(id int) (string, error) {
	JWT, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtClaims{
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(ut.TTLofJWT).Unix(), // TTL of token
			IssuedAt:  time.Now().Unix(),
			Issuer:    "CVmaker-Server",
		},
		id,
	}).SignedString(ut.SignKey)
	if err != nil {
		return "", err
	}

	return JWT, nil
}

func GenerateRT() (string, error) {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

func ValidateJWT(tokenStr string) (*JwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		return ut.SignKey, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			return nil, errors.New(err.Error())
		}
		return nil, errors.New(err.Error())
	}

	claims, ok := token.Claims.(*JwtClaims)
	if !token.Valid {
		return nil, errors.New("Token-is-invalid")
	}
	if !ok {
		return nil, errors.New("Unauthorized")
	}

	return claims, nil
}
