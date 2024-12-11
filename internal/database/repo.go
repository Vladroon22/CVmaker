package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/Vladroon22/CVmaker/internal/ut"

	"github.com/dgrijalva/jwt-go"
)

type Repo struct {
	db  *DataBase
	srv *service.Service
	red *Redis
}

const (
	TTLofJWT = time.Minute * 10
	TTLofCV  = time.Hour * 24 * 7
)

var SignKey = []byte(os.Getenv("KEY"))

type MyClaims struct {
	jwt.StandardClaims
	UserID int
}

func NewRepo(db *DataBase, s *service.Service, r *Redis) *Repo {
	return &Repo{
		db:  db,
		srv: s,
		red: r,
	}
}

func (rp *Repo) Login(device, pass, email string) (int, error) {
	var id int
	var hash string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx, err := rp.db.sqlDB.BeginTx(ctx, &sql.TxOptions{Isolation: 2})
	if err != nil {
		return 0, err
	}

	query1 := "SELECT id, hash_password FROM users WHERE email = $1"
	if err := rp.db.sqlDB.QueryRow(query1, email).Scan(&id, &hash); err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return 0, errors.New("Wrong-password-or-email")
		}
		return 0, err
	}

	query2 := "INSERT INTO sessions (user_id, device_type) VALUES ($1, $2)"
	if _, err := rp.db.sqlDB.Exec(query2, id, device); err != nil {
		tx.Rollback()
		return 0, err
	}

	errTx := tx.Commit()
	if errTx != nil {
		return 0, err
	}

	if err := ut.CheckPassAndHash(hash, pass); err != nil {
		rp.db.logger.Errorln(err)
		return 0, err
	}
	return id, nil
}

func (rp *Repo) GenerateJWT(id int) (string, error) {
	JWT, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &MyClaims{
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(TTLofJWT).Unix(), // TTL of token
			IssuedAt:  time.Now().Unix(),
		},
		id,
	}).SignedString(SignKey)
	if err != nil {
		return "", err
	}

	return JWT, nil
}

func ValidateToken(tokenStr string) (*MyClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &MyClaims{}, func(token *jwt.Token) (interface{}, error) {
		return SignKey, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			return nil, errors.New(err.Error())
		}
		return nil, errors.New(err.Error())
	}

	claims, ok := token.Claims.(*MyClaims)
	if !token.Valid {
		return nil, errors.New("Token-is-invalid")
	}
	if !ok {
		return nil, errors.New("Unauthorized")
	}

	return claims, nil
}

func (rp *Repo) CreateUser(user *service.UserInput) error {
	if err := ut.Valid(user); err != nil {
		rp.db.logger.Errorln(err)
		return err
	}
	enc_pass, err := ut.Hashing(user.Password)
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

func (rp *Repo) AddNewCV(cv *service.CV) error {
	jsonData, err := json.Marshal(cv)
	if err != nil {
		return err
	}
	if err := rp.red.SetData(cv.Profession, string(jsonData), TTLofCV); err != nil {
		return err
	}
	rp.red.Make("lpush", "jobs", cv.Profession)
	rp.db.logger.Infoln("CV successfully added in redis")
	return nil
}

func (rp *Repo) GetProfessions() ([]string, error) {
	prof := []string{}

	data, err := rp.red.Iterate()
	if err != nil {
		rp.db.logger.Errorln(err)
		return nil, err
	}

	for _, item := range data {
		prof = append(prof, item)
		rp.db.logger.Infoln("Redis item: ", item)
	}

	return prof, nil
}

func (rp *Repo) GetDataCV(item string) (*service.CV, error) {
	data, err := rp.red.GetData(item)
	if err != nil {
		return nil, err
	}

	cv := &service.CV{}
	if err := json.Unmarshal([]byte(data), cv); err != nil {
		rp.db.logger.Errorln(err)
		return nil, err
	}

	return cv, nil
}
