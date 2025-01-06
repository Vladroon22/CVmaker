package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	var hash string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	query1 := "SELECT id, hash_password FROM users WHERE email = $1"
	if err := rp.db.sqlDB.QueryRowContext(ctx, query1, email).Scan(&id, &hash); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, errors.New(err.Error())
		}
	}

	if err := ut.CheckPassAndHash(hash, pass); err != nil {
		rp.db.logger.Errorln(err)
		return 0, errors.New("wrong password input")
	}

	return id, nil
}

func (rp *Repo) SaveRT(id int, rt, device string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	query2 := "INSERT INTO sessions (user_id, device_type, refresh_token) VALUES ($1, $2, $3)"
	if _, err := rp.db.sqlDB.ExecContext(ctx, query2, id, device, rt); err != nil {
		return err
	}
	return nil
}

func (rp *Repo) GetRT(id int, providedRT string) error {
	if err := rp.HandleRT(id); err != nil {
		rp.db.logger.Errorln(err)
		return err
	}
	var storedRT string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	query := "SELECT refresh_token FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1"
	if err := rp.db.sqlDB.QueryRowContext(ctx, query, id).Scan(&storedRT); err != nil {
		rp.db.logger.Errorf("no such refresh token for user: %d", id)
		return errors.New("no such refresh token")
	}

	if storedRT != providedRT {
		rp.db.logger.Errorln("tokens not equal to each other")
		return errors.New("tokens not equal to each other")
	}

	return nil
}

func (rp *Repo) HandleRT(id int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx, errTx := rp.db.sqlDB.BeginTx(ctx, &sql.TxOptions{Isolation: 2})
	if errTx != nil {
		rp.db.logger.Errorln(errTx)
		return errors.New("bad start of tx")
	}

	query1 := "SELECT refresh_token, created_at FROM sessions WHERE user_id = $1"
	rows, err := tx.QueryContext(ctx, query1, id)
	if err != nil {
		tx.Rollback()
		rp.db.logger.Errorf("no such refresh token for user: %d\n", id)
		return errors.New("no such refresh token")
	}
	defer rows.Close()

	var counterRT int
	for rows.Next() {
		if !rows.Next() {
			break
		}
		counterRT++
	}

	if err := rows.Err(); err != nil {
		tx.Rollback()
		rp.db.logger.Errorln(err)
		return errors.New(err.Error())
	}

	if counterRT > 6 {
		query2 := "DELETE FROM sessions WHERE user_id = $1 AND created_at < (SELECT MAX(created_at) FROM sessions WHERE user_id = $1"
		if _, err := tx.ExecContext(ctx, query2, id); err != nil {
			tx.Rollback()
			return err
		}
	}

	errTx = tx.Commit()
	if errTx != nil {
		tx.Rollback()
		rp.db.logger.Errorf("failed to commit tx for user: %d", id)
		return errors.New("failed to commit tx")
	}

	return nil
}

func (rp *Repo) CheckExpiredRT(t time.Time) error {
	expTime := t.Add(ut.TTLofRT)
	if time.Now().After(expTime) {
		rp.db.logger.Infoln(ut.ErrTtlExceeded)
		return ut.ErrTtlExceeded
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
func (rp *Repo) GetProfessions(id int) ([]string, error) {
	professions := []string{}

	jobs, err := rp.red.Iterate()
	if err != nil {
		rp.db.logger.Errorln("Error of fetching jobs: ", err)
		return nil, err
	}

	chJobs := make(chan string)
	go func() { // add WaitGroup for more num of jobs
		defer close(chJobs)
		for _, job := range jobs {
			cv, err := rp.GetDataCV(job)
			if err != nil {
				rp.db.logger.Errorln("Error of fetching jobs: ", err)
				continue
			}
			if cv.ID == id {
				chJobs <- job
			}
		}
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
