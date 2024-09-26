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
