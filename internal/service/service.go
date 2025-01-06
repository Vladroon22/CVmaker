package service

import "time"

type UserInput struct {
	Name     string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CV struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Age         int      `json:"age"`
	Profession  string   `json:"profession"`
	Surname     string   `json:"surname"`
	EmailCV     string   `json:"emailcv"`
	LivingCity  string   `json:"city"`
	Salary      int      `json:"salary"`
	PhoneNumber string   `json:"phone"`
	Skills      []string `json:"skills"`
	Education   string   `json:"education"`
}

type Utils interface {
	CheckPassAndHash(string, string) error
	Hashing(string) ([]byte, error)
	ValidateEmail(string) bool
	Valid(*UserInput) error
	ValidatePhone(string) bool
	ValidateDataAge(string) bool
	CountUserAge(time.Time) int
}

type Service struct {
	CV
	UserInput
}

func NewService() *Service {
	return &Service{}
}
