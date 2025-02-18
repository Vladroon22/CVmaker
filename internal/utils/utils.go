package utils

import (
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	ent "github.com/Vladroon22/CVmaker/internal/entity"
	"golang.org/x/crypto/bcrypt"
)

const (
	TTLofJWT = time.Minute * 10
	TTLofCV  = time.Hour * 24 * 7
)

var SignKey = []byte(os.Getenv("KEY"))

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

func Valid(user *ent.UserInput) error {
	if ok := ValidateEmail(user.Email); !ok {
		return errors.New("wrong email input")
	}
	if len(user.Password) < 7 {
		return errors.New("password must be more than 7 symbols")
	}
	if len(user.Name) == 0 {
		return nil
	} else if len(user.Name) > 100 {
		return errors.New("name is too long")
	}
	return nil
}

func BinSearch(cvs []ent.CV, goal int, prof string) (ent.CV, bool) {
	if len(cvs) == 0 {
		return ent.CV{}, false
	}

	sort.Slice(cvs, func(i, j int) bool { return cvs[i].ID < cvs[j].ID })

	beg := 0
	end := len(cvs) - 1

	for beg <= end {
		mid := beg + (end-beg)/2
		if cvs[mid].ID == goal && cvs[mid].Profession == prof {
			return cvs[mid], true
		} else if cvs[mid].ID < goal {
			beg = mid + 1
		} else {
			end = mid - 1
		}
	}
	return ent.CV{}, false
}

func BinSearchIndex(cvs []ent.CV, id int, prof string) int {
	sort.Slice(cvs, func(i, j int) bool { return cvs[i].ID < cvs[j].ID })
	i := sort.Search(len(cvs), func(i int) bool { return cvs[i].ID == id && cvs[i].Profession == prof })
	if i < 0 || i > len(cvs) && cvs[i].Profession == prof {
		return i
	}
	return -1
}
