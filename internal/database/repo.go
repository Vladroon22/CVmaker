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
	query := "SELECT id, hash_password FROM users WHERE email = $1"
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

func (rp *Repo) CreateUser(name, password, email string) error {
	user := rp.srv.UserInput
	user.Name = name
	user.Email = email
	user.Password = password
	if err := Valid(&user); err != nil {
		return err
	}
	enc_pass, err := Hashing(password)
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

func (rp *Repo) AddNewCV(age, salary int, profession, name, surname, email_cv, city, phone, education string, skills ...string) (int, error) {
	cv := rp.srv.CV
	cv.Name = name
	cv.Age = age
	cv.Surname = surname
	cv.PhoneNumber = phone
	cv.LivingCity = city
	cv.Salary = salary
	cv.Education = education

	var id int
	query := "INSERT INTO CVs (profession, name, age, surname, email_cv, living_city, salary, phone_number, education, skills) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"
	if _, err := rp.db.sqlDB.Exec(query, cv.Profession, cv.Name, cv.Age, cv.Surname, cv.EmailCV, cv.LivingCity, cv.Salary, cv.PhoneNumber, cv.Education, cv.Skills); err != nil {
		rp.db.logger.Errorln(err)
		return 0, err
	}

	rp.db.logger.Infoln("User successfully added")
	return id, nil
}

func (rp *Repo) GetDataCV(id int) (*service.CV, error) {
	cv := service.CV{}
	query := "SELECT profession, name, age surname, email_cv, living_city, salary, phone_number, education FROM CVs WHERE id = $1"

	err := rp.db.sqlDB.QueryRow(query, id).Scan(&cv.Profession, &cv.Name, &cv.Age, &cv.Surname, &cv.EmailCV, &cv.LivingCity, &cv.Salary, &cv.PhoneNumber, &cv.Education)
	if err != nil {
		rp.db.logger.Errorln(err)
		return nil, err
	}

	queryS := "SELECT skills FROM CVs WHERE id = $1"

	rows, err := rp.db.sqlDB.Query(queryS, id)
	if err != nil {
		rp.db.logger.Errorln(err)
		return nil, err
	}

	// Срез для хранения навыков
	var skills []string

	// Извлечение данных из результатов запроса
	for rows.Next() {
		var skill string
		if err := rows.Scan(&skill); err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	// Проверка на наличие ошибок после итерации
	if err := rows.Err(); err != nil {
		rp.db.logger.Errorln(rows.Err())
		return nil, err
	}
	cv.Skills = skills

	return &cv, nil
}
