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

func validateEmail(email string) bool {
	emailRegex := regexp.MustCompile("(^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+.[a-zA-Z0-9-.]+$)")
	return emailRegex.MatchString(email)
}

func Valid(user *service.UserInput) error {
	if user.Password == "" {
		return errors.New("password cant't be blank")
	} else if len(user.Password) >= 50 {
		return errors.New("password cant't be more than 50 symbols")
	} else if user.Name == "" {
		return errors.New("username cant't be blank")
	} else if user.Email == "" {
		return errors.New("email can't be blank")
	}

	if ok := validateEmail(user.Email); !ok {
		return errors.New("wrong email input")
	}
	return nil
}
