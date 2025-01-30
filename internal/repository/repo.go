package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/Vladroon22/CVmaker/internal/database"
	ent "github.com/Vladroon22/CVmaker/internal/entity"
	"github.com/Vladroon22/CVmaker/internal/utils"
	golog "github.com/Vladroon22/GoLog"
	"github.com/jackc/pgx/v5"
	pool "github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	db   *pool.Pool
	red  *database.Redis
	logg *golog.Logger
}

func NewRepo(db *pool.Pool, lg *golog.Logger, r *database.Redis) Repo {
	return Repo{
		db:   db,
		logg: lg,
		red:  r,
	}
}

func (rp *Repo) Login(c context.Context, pass, email string) (int, error) {
	var id int
	var hash, storedEmail string

	ctx, cancel := context.WithCancel(c)
	defer cancel()

	query1 := "SELECT id, email, hash_password FROM users WHERE email = $1"
	if err := rp.db.QueryRow(ctx, query1, email).Scan(&id, &storedEmail, &hash); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			rp.logg.Errorln("bad resp: ", err)
			return 0, errors.New("bad response from database")
		}
	}

	if storedEmail != email {
		rp.logg.Errorln("no such user's email")
		return 0, errors.New("no such user's email")
	}

	if err := utils.CheckPassAndHash(hash, pass); err != nil {
		rp.logg.Errorln(err)
		return 0, errors.New("wrong password input")
	}

	return id, nil
}

func (rp *Repo) SaveSession(c context.Context, id int, ip, device string) error {
	ctx, cancel := context.WithCancel(c)
	defer cancel()

	tx, errTx := rp.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if errTx != nil {
		rp.logg.Errorln("Beg Tx (session): ", errTx)
		return errors.New("bad response from database")
	}

	defer func() {
		errRb := tx.Rollback(ctx)
		if errRb != nil && !errors.Is(errRb, pgx.ErrTxClosed) {
			rp.logg.Errorln("Rollback Tx (session): ", errRb)
		}
	}()

	var cnt int
	query1 := "SELECT COUNT(*) FROM sessions WHERE user_id = $1"
	if err := tx.QueryRow(ctx, query1, id).Scan(&cnt); err != nil {
		rp.logg.Errorln("Tx to select (session): ", err)
		return errors.New("bad response from database")
	}

	if cnt >= 4 {
		query2 := "DELETE FROM sessions WHERE user_id = $1"
		if _, err := tx.Exec(ctx, query2, id); err != nil {
			rp.logg.Errorln("Tx to delete (session): ", err)
			return errors.New("bad response from database")
		}
	}

	query3 := "INSERT INTO sessions (user_id, device_type, ip, created_at) VALUES ($1, $2, $3, $4)"
	if _, err := tx.Exec(ctx, query3, id, device, ip, time.Now().UTC()); err != nil {
		rp.logg.Errorln("Tx to insert (session): ", errTx)
		return errors.New("bad response from database")
	}

	errTx = tx.Commit(ctx)
	if errTx != nil {
		rp.logg.Errorln("failed to commit tx (session): ", errTx)
		return errors.New("bad response from database")
	}
	rp.logg.Infoln("User successfully log in")
	return nil
}

func (rp *Repo) CreateUser(c context.Context, user *ent.UserInput) error {
	var emailStored string

	ctx, cancel := context.WithCancel(c)
	defer cancel()

	tx, errTx := rp.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if errTx != nil {
		rp.logg.Errorln("Beg Tx (create user): ", errTx)
		return errTx
	}

	defer func() {
		errRb := tx.Rollback(ctx)
		if errRb != nil && !errors.Is(errRb, pgx.ErrTxClosed) {
			rp.logg.Errorln("Rollback Tx (session): ", errRb)
		}
	}()

	query1 := "SELECT email FROM users WHERE email = $1"
	if errRows := tx.QueryRow(ctx, query1, user.Email).Scan(&emailStored); errRows != nil {
		if !errors.Is(errRows, sql.ErrNoRows) {
			rp.logg.Errorln("bad resp (rows): ", errRows)
			return errors.New("bad response from database")
		}
	}

	if emailStored == user.Email {
		rp.logg.Errorln("such user's email allready existed")
		return errors.New("such user's email allready existed")
	}

	enc_pass, err := utils.Hashing(user.Password)
	if err != nil {
		rp.logg.Errorln(err)
		return errors.New("hashing password error")
	}

	query := "INSERT INTO users (name, email, hash_password) VALUES ($1, $2, $3)"
	if _, err := tx.Exec(ctx, query, user.Name, user.Email, string(enc_pass)); err != nil {
		rp.logg.Errorln("Tx to insert (create user): ", err)
		return errors.New("bad response from database")
	}

	errTx = tx.Commit(ctx)
	if errTx != nil {
		rp.logg.Errorln("failed to commit tx (create user): ", user.Name)
		return errors.New("bad response from database")
	}

	rp.logg.Infoln("User successfully added")
	return nil
}

func (rp *Repo) AddNewCV(cv *ent.CV) error {
	jsonData, err := json.Marshal(cv)
	if err != nil {
		rp.logg.Errorln(err)
		return err
	}
	if err := rp.red.SetData(cv.Profession, string(jsonData), utils.TTLofCV); err != nil {
		rp.logg.Errorln(err)
		return err
	}
	rp.red.Make("lpush", "jobs", cv.Profession)
	rp.logg.Infoln("CV successfully added in redis")
	return nil
}

func (rp *Repo) GetProfessions(id int) ([]string, error) {
	professions := []string{}

	jobs, err := rp.red.Iterate()
	if err != nil {
		return nil, err
	}

	chJobs := make(chan string, 10)
	wg := &sync.WaitGroup{}
	for _, job := range jobs {
		wg.Add(1)
		go func(job string) {
			defer wg.Done()
			cv, err := rp.GetDataCV(job)
			if err != nil {
				return
			}
			if cv == nil {
				return
			}
			if cv.ID == id {
				chJobs <- job
			}
		}(job)
	}

	go func() {
		wg.Wait()
		close(chJobs)
	}()

	for job := range chJobs {
		professions = append(professions, job)
	}

	return professions, nil
}

func (rp *Repo) GetDataCV(item string) (*ent.CV, error) {
	data, err := rp.red.GetData(item)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}

	cv := &ent.CV{}
	if err := json.Unmarshal([]byte(data), cv); err != nil {
		rp.logg.Errorln(err)
		return nil, err
	}

	return cv, nil
}
