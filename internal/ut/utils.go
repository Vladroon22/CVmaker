package ut

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Vladroon22/CVmaker/internal/service"
	"golang.org/x/crypto/bcrypt"
)

const (
	TTLofJWT = time.Minute * 20
	TTLofCV  = time.Hour * 24 * 7
)

var (
	SignKey        = []byte(os.Getenv("KEY"))
	ErrTtlExceeded = errors.New("ttl of rt exceeded")
	ErrRTsMoreFive = errors.New("too many rts for one user")
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

func ValidatePhone(phone string) bool {
	phoneRegex := regexp.MustCompile(`^(?:\+7|8)?[\s-]?\(?\d{3}\)?[\s-]?\d{2,3}[\s-]?\d{2,3}[\s-]?\d{2,4}$`)
	return phoneRegex.MatchString(phone)
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func ValidateDataAge(data string) bool {
	dateRegex := regexp.MustCompile(`^(0[1-9]|[12][0-9]|3[01])\.(0[1-9]|1[0-2])\.(19|20)\d{2}$`)
	if !dateRegex.MatchString(data) {
		return false
	}

	parts := strings.Split(data, ".")
	day, month, year := parts[0], parts[1], parts[2]

	dayInt := int(day[0]-'0')*10 + int(day[1]-'0')
	monthInt := int(month[0]-'0')*10 + int(month[1]-'0')
	yearInt := int(year[0]-'0')*1000 + int(year[1]-'0')*100 + int(year[2]-'0')*10 + int(year[3]-'0')

	switch monthInt {
	case 2: // February
		if isLeapYear(yearInt) {
			return dayInt <= 29
		}
		return dayInt <= 28
	case 4, 6, 9, 11: // April, June, September, November
		return dayInt <= 30
	default: // rest mouths
		return dayInt <= 31
	}
}

func CountUserAge(userAge time.Time) int {
	currTime := time.Now()
	currAge := currTime.Year() - userAge.Year()

	if currTime.Month() < userAge.Month() || currTime.Month() == userAge.Month() && currTime.Day() < userAge.Day() {
		currAge--
	}

	return currAge
}

func Valid(user *service.UserInput) error {
	if ok := ValidateEmail(user.Email); !ok {
		return errors.New("wrong email input")
	}
	if len(user.Password) < 7 || len(user.Password) >= 70 {
		return errors.New("password must be more than 7 and less than 70 symbols")
	}
	if len(user.Name) == 0 {
		return nil
	} else if len(user.Name) > 100 {
		return errors.New("name is too long")
	} else if len(user.Name) < 3 || len(user.Name) >= 70 {
		return errors.New("username must be more than 3 and less than 70 symbols")
	}
	return nil
}
