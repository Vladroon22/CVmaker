package database

import (
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/Vladroon22/CVmaker/config"
	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/dgrijalva/jwt-go"
)

var SignKey = os.Getenv("JWT")

type MyClaims struct {
	jwt.StandardClaims
	UserId int `json:"id"`
}

type Repo struct {
	db   *DataBase
	conf *config.Config
	srv  *service.Service
}

func NewRepo(db *DataBase, cnf *config.Config, s *service.Service) *Repo {
	return &Repo{
		db:   db,
		conf: cnf,
		srv:  s,
	}
}

func (rp *Repo) GenerateJWT(pass, email string) (string, error) {
	var id int
	var hash string
	query := "SELECT id, hash_password FROM clients WHERE email = $1"
	if err := rp.db.sqlDB.QueryRow(query, email).Scan(&id, &hash); err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("No-such-user")
		}
		return "", err
	}

	if err := CheckPassAndHash(hash, pass); err != nil {
		rp.db.logger.Errorln(err)
		return "", err
	}

	JWT, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &MyClaims{
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(5 * time.Hour).Unix(), // TTL of token
			IssuedAt:  time.Now().Unix(),
		},
		id,
	}).SignedString([]byte(SignKey))
	if err != nil {
		return "", err
	}

	return JWT, nil
}

func (rp *Repo) CreateUser(name, password, email string) (int, error) {
	user := &rp.srv.UserInput
	if err := Valid(user); err != nil {
		return 0, err
	}
	enc_pass, err := Hashing(password)
	if err != nil {
		rp.db.logger.Errorln(err)
		return 0, err
	}
	var id int
	query := "INSERT INTO clients (username, email, encrypt_password) VALUES ($1, $2, $3) RETURNING id"
	if err := rp.db.sqlDB.QueryRow(query, user.Name, user.Email, enc_pass).Scan(&id); err != nil {
		rp.db.logger.Errorln(err)
		return 0, err
	}

	rp.db.logger.Infoln("User successfully added")
	return id, nil
}

func (rp *Repo) AddNewCV(salary int, name, surname, email_cv, city, phone, education string) (int, error) {
	cv := &rp.srv.CV
	cv.Name = name
	cv.Surname = surname
	cv.PhoneNumber = phone
	cv.LivingCity = city
	cv.Salary = salary
	cv.Education = education

	var id int
	query := "INSERT INTO CVs (name, surname, email_cv, living_city, salary, phone_number, education) VALUES ($1, $2, $3, $4, $5, $6, $7)"
	if _, err := rp.db.sqlDB.Exec(query, cv.Name, cv.Surname, cv.EmailCV, cv.LivingCity, cv.Salary, cv.PhoneNumber, cv.Education); err != nil {
		rp.db.logger.Errorln(err)
		return 0, err
	}

	rp.db.logger.Infoln("User successfully added")
	return id, nil
}

func (rp *Repo) InsertSkills(cvID int, skills ...string) error {
	q1 := "INSERT INTO Skills (skill_name) VALUES ($1) RETURNING id"
	q2 := "INSERT INTO CV_Skills (cv_id, skill_id) VALUES ($1, $2)"

	tx, err := rp.db.sqlDB.Begin()
	if err != nil {
		return err
	}

	for _, skill := range skills {
		var skillID int
		err := tx.QueryRow(q1, skill).Scan(&skillID)
		if err != nil {
			tx.Rollback()
			return err
		}
		_, err = tx.Exec(q2, cvID, skillID)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
