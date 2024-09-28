package database

import (
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/dgrijalva/jwt-go"
	"github.com/lib/pq"
)

type Repo struct {
	db  *DataBase
	srv *service.Service
}

var SignKey = os.Getenv("JWT")

type MyClaims struct {
	jwt.StandardClaims
	UserID int `json:"id"`
}

func NewRepo(db *DataBase, s *service.Service) *Repo {
	return &Repo{
		db:  db,
		srv: s,
	}
}

func (rp *Repo) Login(pass, email string) (int, error) {
	var id int
	var hash string
	query := "SELECT id, hash_password FROM users WHERE email = $1"
	if err := rp.db.sqlDB.QueryRow(query, email).Scan(&id, &hash); err != nil {
		if err == sql.ErrNoRows {
			return 0, errors.New("No-such-user")
		}
		return 0, err
	}

	if err := CheckPassAndHash(hash, pass); err != nil {
		rp.db.logger.Errorln(err)
		return 0, err
	}
	return id, nil
}

func (db *Repo) GenerateJWT(id int, pass, email string) (string, error) {
	JWT, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &MyClaims{
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour).Unix(), // TTL of token
			IssuedAt:  time.Now().Unix(),
		},
		id,
	}).SignedString([]byte(SignKey))
	if err != nil {
		return "", err
	}

	return JWT, nil
}

func (rp *Repo) CreateUser(user *service.UserInput) error {
	if err := Valid(user); err != nil {
		rp.db.logger.Errorln(err)
		return err
	}
	enc_pass, err := Hashing(user.Password)
	if err != nil {
		rp.db.logger.Errorln(err)
		return err
	}
	query := "INSERT INTO users (name, email, hash_password) VALUES ($1, $2, $3)"
	if _, err := rp.db.sqlDB.Exec(query, user.Name, user.Email, string(enc_pass)); err != nil {
		rp.db.logger.Errorln(err)
		return err
	}

	rp.db.logger.Infoln("User successfully added")
	return nil
}

func (rp *Repo) AddNewCV(cv *service.CV) (int, error) {
	var id int

	query := "INSERT INTO CVs (profession, name, age, surname, email_cv, living_city, salary, phone_number, education, skills) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"
	if _, err := rp.db.sqlDB.Exec(query, cv.Profession, cv.Name, cv.Age, cv.Surname, cv.EmailCV, cv.LivingCity, cv.Salary, cv.PhoneNumber, cv.Education, pq.Array(cv.Skills)); err != nil {
		rp.db.logger.Errorln(err)
		return 0, err
	}

	rp.db.logger.Infoln("New CV successfully added")
	return id, nil
}

func (rp *Repo) GetDataCV(id int) (*service.CV, error) {
	cv := service.CV{}
	query := "SELECT id profession, name, age surname, email_cv, living_city, salary, phone_number, education FROM CVs WHERE id = $1"
	var skills []string

	err := rp.db.sqlDB.QueryRow(query, id).Scan(&cv.ID, &cv.Name, &cv.Age, &cv.Surname, &cv.EmailCV, &cv.LivingCity, &cv.Profession, &cv.Salary, &cv.PhoneNumber, &cv.Education, pq.Array(&skills))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	cv.Skills = skills

	return &cv, nil
}

func (rp *Repo) handlerRows() (bool, error) {
	var count int
	countQuery := "SELECT COUNT(*) FROM CVs"
	err := rp.db.sqlDB.QueryRow(countQuery).Scan(&count)
	if err != nil {
		return false, err
	}
	var lastID int
	query := "SELECT id FROM CVs ORDER BY id DESC LIMIT 1"
	err = rp.db.sqlDB.QueryRow(query).Scan(&lastID)
	if err != nil {
		return false, err
	}
	if count == lastID {
		return false, nil
	}
	return true, nil
}

func (rp *Repo) CheckDB() ([]*service.CV, error) {
	ok, err := rp.handlerRows()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	var cvs []*service.CV
	query := "SELECT id, name, age, surname, email_cv, living_city, profession, salary, phone_number, education, skills FROM CVs"

	rows, err := rp.db.sqlDB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		cv := service.CV{}
		var skills []string

		err := rows.Scan(
			&cv.ID,
			&cv.Name,
			&cv.Age,
			&cv.Surname,
			&cv.EmailCV,
			&cv.LivingCity,
			&cv.Profession,
			&cv.Salary,
			&cv.PhoneNumber,
			&cv.Education,
			pq.Array(&skills),
		)
		if err != nil {
			return nil, err
		}

		cv.Skills = skills

		cvs = append(cvs, &cv)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return cvs, nil
}
