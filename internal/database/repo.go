package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/dgrijalva/jwt-go"
)

type Repo struct {
	db  *DataBase
	srv *service.Service
	red *Redis
}

var SignKey = os.Getenv("JWT")

type MyClaims struct {
	jwt.StandardClaims
	UserID int `json:"id"`
}

func NewRepo(db *DataBase, s *service.Service, r *Redis) *Repo {
	return &Repo{
		db:  db,
		srv: s,
		red: r,
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

func (rp *Repo) AddNewCV(cv *service.CV) error {
	jsonData, err := json.Marshal(cv)
	if err != nil {
		return err
	}
	rp.db.logger.Infoln(string(jsonData))
	if err := rp.red.SetData(cv.Profession, string(jsonData), time.Minute*30); err != nil {
		return err
	}
	rp.red.Make("lpush", "jobs", cv.Profession)
	rp.db.logger.Infoln("CV successfully added")
	return nil
}

func (rp *Repo) GetProfessionCV() ([]string, error) {
	prof := []string{}

	data, err := rp.red.Iterate()
	rp.db.logger.Infoln(data)
	if err != nil {
		rp.db.logger.Errorln(err)
		return nil, err
	}

	for _, item := range data {
		prof = append(prof, item)
		rp.db.logger.Infoln("Item: ", item)
	}

	rp.db.logger.Infoln("Profession iterated")
	return prof, nil
}

func (rp *Repo) GetDataCV(item string) (service.CV, error) {
	cv := service.CV{}
	data := rp.red.GetData(item)
	rp.db.logger.Infoln(data)

	if err := json.Unmarshal([]byte(data), &cv); err != nil {
		rp.db.logger.Errorln(err)
		return service.CV{}, err
	}

	rp.db.logger.Infoln("Get the item")
	return cv, nil
}
