package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Vladroon22/CVmaker/internal/database"
	ent "github.com/Vladroon22/CVmaker/internal/entity"
	"github.com/Vladroon22/CVmaker/internal/utils"
	"github.com/jackc/pgx/v5"
)

type Repo struct {
	db  *database.DataBase
	red *database.Redis
}

func NewRepo(db *database.DataBase, r *database.Redis) *Repo {
	return &Repo{
		db:  db,
		red: r,
	}
}

func (rp *Repo) Login(c context.Context, pass, email string) (int, error) {
	ctx, cancel := context.WithTimeout(c, time.Second*15)
	defer cancel()

	var (
		id                int
		hash, storedEmail string
	)

	pool := rp.db.GetPool()

	query1 := "SELECT id, email, hash_password FROM users WHERE email = $1"
	if err := pool.QueryRow(ctx, query1, email).Scan(&id, &storedEmail, &hash); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Println("bad resp: ", err)
			return 0, errors.New("bad response from database")
		}
	}

	if storedEmail != email {
		log.Println("no such user's email")
		return 0, errors.New("no such user's email")
	}

	if err := utils.CheckPassAndHash(hash, pass); err != nil {
		log.Println(err)
		return 0, errors.New("wrong password input")
	}

	return id, nil
}

func (rp *Repo) SaveSession(c context.Context, id int, device string) error {
	ctx, cancel := context.WithTimeout(c, time.Second*15)
	defer cancel()

	pool := rp.db.GetPool()

	tx, errTx := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if errTx != nil {
		log.Println("Beg Tx (session): ", errTx)
		return errors.New("bad response from database")
	}

	defer func() {
		errRb := tx.Rollback(ctx)
		if errRb != nil && !errors.Is(errRb, pgx.ErrTxClosed) {
			log.Println("Rollback Tx (session): ", errRb)
		}
	}()

	var cnt int
	query1 := "SELECT COUNT(*) FROM sessions WHERE user_id = $1"
	if err := tx.QueryRow(ctx, query1, id).Scan(&cnt); err != nil {
		log.Println("Tx to select (session): ", err)
		return errors.New("bad response from database")
	}

	if cnt >= 4 {
		query2 := "DELETE FROM sessions WHERE user_id = $1"
		if _, err := tx.Exec(ctx, query2, id); err != nil {
			log.Println("Tx to delete (session): ", err)
			return errors.New("bad response from database")
		}
	}

	query3 := "INSERT INTO sessions (user_id, device_type, created_at) VALUES ($1, $2, $3)"
	if _, err := tx.Exec(ctx, query3, id, device, time.Now().UTC()); err != nil {
		log.Println("Tx to insert (session): ", errTx)
		return errors.New("bad response from database")
	}

	if err := tx.Commit(ctx); err != nil {
		log.Println("failed to commit tx (session): ", errTx)
		return errors.New("bad response from database")
	}

	log.Println("User successfully log in")

	return nil
}

func (rp *Repo) CreateUser(c context.Context, user *ent.UserInput) error {
	ctx, cancel := context.WithTimeout(c, time.Second*15)
	defer cancel()

	var emailStored string

	pool := rp.db.GetPool()

	tx, errTx := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if errTx != nil {
		log.Println("Beg Tx (create user): ", errTx)
		return errTx
	}

	defer func() {
		errRb := tx.Rollback(ctx)
		if errRb != nil && !errors.Is(errRb, pgx.ErrTxClosed) {
			log.Println("Rollback Tx (session): ", errRb)
		}
	}()

	query1 := "SELECT email FROM users WHERE email = $1"
	if errRows := tx.QueryRow(ctx, query1, user.Email).Scan(&emailStored); errRows != nil {
		if !errors.Is(errRows, sql.ErrNoRows) {
			log.Println("bad resp (rows): ", errRows)
			return errors.New("bad response from database")
		}
	}

	if emailStored == user.Email {
		log.Println("such user's email allready existed")
		return errors.New("such user's email allready existed")
	}

	enc_pass, err := utils.Hashing(user.Password)
	if err != nil {
		log.Println(err)
		return errors.New("hashing password error")
	}

	query := "INSERT INTO users (name, email, hash_password) VALUES ($1, $2, $3)"
	if _, err := tx.Exec(ctx, query, user.Name, user.Email, string(enc_pass)); err != nil {
		log.Println("Tx to insert (create user): ", err)
		return errors.New("bad response from database")
	}

	if err := tx.Commit(ctx); err != nil {
		log.Println("failed to commit tx (create user): ", user.Name)
		return errors.New("bad response from database")
	}

	log.Println("User successfully added")
	return nil
}

func (rp *Repo) AddNewCV(cv *ent.CV) error {
	jsonData, err := json.Marshal(cv)
	if err != nil {
		log.Println(err)
		return err
	}
	key := fmt.Sprintf("job:%s:id:%d", cv.Profession, cv.ID)
	if err := rp.red.SetData(key, string(jsonData), utils.TTLofCV); err != nil {
		log.Println(err)
		return err
	}
	//jobEntry := fmt.Sprintf("%s:%d", cv.Profession, cv.ID)
	//rp.red.Make("lpush", "jobs", jobEntry)
	log.Println("CV successfully added in redis")
	return nil
}

func (rp *Repo) GetProfessions(id int) ([]string, error) {
	professions := []string{}

	jobs, _ := rp.red.IterateWithPattern("job:*:id:*")
	log.Println(jobs)

	chJobs := make(chan string, len(jobs))
	wg := &sync.WaitGroup{}
	for _, job := range jobs {
		wg.Add(1)
		go func(job string) {
			log.Println(job)
			defer wg.Done()
			cv, err := rp.GetDataCV(id, job)
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

func (rp *Repo) GetDataCV(id int, prof string) (*ent.CV, error) {
	keys := strings.Split(prof, ":")
	key := fmt.Sprintf("job:%s:id:%s", keys[1], keys[3])

	data, err := rp.red.GetData(key)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, fmt.Errorf("data is empty")
	}

	cv := &ent.CV{}
	if err := json.Unmarshal([]byte(data), cv); err != nil {
		return nil, err
	}

	return cv, nil
}

func (rp *Repo) DeleteCV(id int, prof string) error {
	key := fmt.Sprintf("job:%s:id:%d", prof, id)
	rp.red.Make("del", key)
	return nil
}
