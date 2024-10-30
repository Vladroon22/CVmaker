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

const (
	TTLofJWT = time.Hour * 24
	TTLofCV  = time.Hour * 24 * 7
)

var SignKey = []byte(os.Getenv("KEY"))

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

func (rp *Repo) GenerateJWT(id int, pass, email string) (string, error) {
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
	if err := rp.red.SetData(cv.Profession, string(jsonData), TTLofCV); err != nil {
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
		rp.db.logger.Infoln("Redis item: ", item)
	}

	rp.db.logger.Infoln("Profession iterated")
	return prof, nil
}

func (rp *Repo) GetDataCV(item string) (*service.CV, error) {
	data, err := rp.red.GetData(item)
	if err != nil {
		return nil, err
	}
	rp.db.logger.Infoln(data)

	cv := &service.CV{}
	if err := json.Unmarshal([]byte(data), cv); err != nil {
		rp.db.logger.Errorln(err)
		return nil, err
	}

	rp.db.logger.Infoln("Get the item")
	return cv, nil
}
