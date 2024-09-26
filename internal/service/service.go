package service

type UserInput struct {
	Name     string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"pass"`
}

type CV struct{}

type Service struct {
	CV
	UserInput
}

func NewService() *Service {
	return &Service{}
}
