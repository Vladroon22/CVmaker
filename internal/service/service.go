package service

type UserInput struct {
	Name     string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CV struct {
	ID          int      `json:"cv_id"`
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
}

type Tx interface {
	GetDataCV(id int) (*CV, error)
	AddNewCV(*CV) (int, error)
	CreateUser(*UserInput) error
	GenerateJWT(int, string, string) (string, error)
	Login(string, string) (int, error)
}

type Service struct {
	CV
	UserInput
	Tx
	Utils
}

func NewService() *Service {
	return &Service{}
}
