package database

import (
	"errors"
	"regexp"

	"github.com/Vladroon22/CVmaker/internal/service"
	"golang.org/x/crypto/bcrypt"
)

func CheckPassAndHash(hash, pass string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)); err != nil {
		return err
	}
	return nil
}

func Hashing(pass string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
}

func ValidateEmail(email string) bool {
	emailRegex := regexp.MustCompile("(^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+.[a-zA-Z0-9-.]+$)")
	return emailRegex.MatchString(email)
}

func Valid(user *service.UserInput) error {
	if ok := ValidateEmail(user.Email); !ok {
		return errors.New("wrong email input")
	}
	if len(user.Password) <= 7 || len(user.Password) >= 70 {
		return errors.New("password must be more than 7 and less than 70 symbols")
	}
	if len(user.Name) == 0 {
		return nil
	} else if len(user.Name) <= 3 || len(user.Name) >= 70 {
		return errors.New("username must be more than 3 and less than 70 symbols")
	}
	return nil
}
