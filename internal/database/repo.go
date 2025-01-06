package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/Vladroon22/CVmaker/internal/ut"
)

type Repo struct {
	db  *DataBase
	srv *service.Service
	red *Redis
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
	var hash, storedEmail string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	query1 := "SELECT id, email, hash_password FROM users WHERE email = $1"
	if err := rp.db.sqlDB.QueryRowContext(ctx, query1, email).Scan(&id, &storedEmail, &hash); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, errors.New(err.Error())
		}
	}

	if storedEmail != email {
		rp.db.logger.Errorln("no such user's email")
		return 0, errors.New("no such user's email")
	}

	if err := ut.CheckPassAndHash(hash, pass); err != nil {
		rp.db.logger.Errorln(err)
		return 0, errors.New("wrong password input")
	}

	return id, nil
}

func (rp *Repo) SaveSession(id int, device string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	query2 := "INSERT INTO sessions (user_id, device_type, created_at) VALUES ($1, $2, $3)"
	if _, err := rp.db.sqlDB.ExecContext(ctx, query2, id, device, time.Now().UTC()); err != nil {
		return err
	}
	return nil
}

func (rp *Repo) CreateUser(user *service.UserInput) error {
	var emailStored string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx, errTx := rp.db.sqlDB.BeginTx(ctx, &sql.TxOptions{Isolation: 2})
	if errTx != nil {
		rp.db.logger.Errorln(errTx)
		return errTx
	}

	query1 := "SELECT email FROM users WHERE email = $1"
	if errRows := tx.QueryRowContext(ctx, query1, user.Email).Scan(&emailStored); errRows != nil {
		if !errors.Is(errRows, sql.ErrNoRows) {
			tx.Rollback()
			return errors.New(errRows.Error())
		}
	}

	if emailStored == user.Email {
		tx.Rollback()
		return errors.New("such user's email allready existed")
	}

	enc_pass, err := ut.Hashing(user.Password)
	if err != nil {
		tx.Rollback()
		rp.db.logger.Errorln(err)
		return err
	}

	query := "INSERT INTO users (name, email, hash_password) VALUES ($1, $2, $3)"
	if _, err := tx.ExecContext(ctx, query, user.Name, user.Email, string(enc_pass)); err != nil {
		tx.Rollback()
		rp.db.logger.Errorln(err)
		return err
	}

	errTx = tx.Commit()
	if errTx != nil {
		tx.Rollback()
		rp.db.logger.Errorln("failed to commit tx for user: " + user.Name)
		return errors.New("failed to commit tx")
	}

	rp.db.logger.Infoln("User successfully added")
	return nil
}

func (rp *Repo) AddNewCV(cv *service.CV) error {
	jsonData, err := json.Marshal(cv)
	if err != nil {
		rp.red.logger.Errorln(err)
		return err
	}
	if err := rp.red.SetData(cv.Profession, string(jsonData), ut.TTLofCV); err != nil {
		rp.red.logger.Errorln(err)
		return err
	}
	rp.red.Make("lpush", "jobs", cv.Profession)
	rp.db.logger.Infoln("CV successfully added in redis")
	return nil
}

func (rp *Repo) jobHandler(chJobs chan<- string, id int, job string) error {
	cv, err := rp.GetDataCV(job)
	if err != nil {
		return err
	}

	if cv == nil {
		return nil
	}

	if cv.ID == id {
		chJobs <- job
	}
	return nil
}

func (rp *Repo) GetProfessions(id int) ([]string, error) {
	professions := []string{}

	jobs, err := rp.red.Iterate()
	if err != nil {
		rp.db.logger.Errorln("Error of fetching jobs: ", err)
		return nil, err
	}

	chJobs := make(chan string)
	wg := &sync.WaitGroup{}
	for _, job := range jobs {
		defer wg.Done()
		wg.Add(1)
		go rp.jobHandler(chJobs, id, job)
	}

	go func() {
		close(chJobs)
		wg.Wait()
	}()

	for job := range chJobs {
		professions = append(professions, job)
		rp.db.logger.Infoln("Redis item: ", job)
	}

	return professions, nil
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
