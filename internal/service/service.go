package service

type UserInput struct {
	Name     string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CV struct {
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

type Service struct {
	CV
	UserInput
}

func NewService() *Service {
	return &Service{}
}
