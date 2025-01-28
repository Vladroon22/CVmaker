package service

import (
	"context"

	ent "github.com/Vladroon22/CVmaker/internal/entity"
	"github.com/Vladroon22/CVmaker/internal/repository"
)

type Servicer interface {
	SaveSession(c context.Context, id int, ip, device string) error
	Login(c context.Context, pass, email string) (int, error)
	CreateUser(c context.Context, user *ent.UserInput) error
	GetProfessions(id int) ([]string, error)
	GetDataCV(item string) (*ent.CV, error)
	AddNewCV(cv *ent.CV) error
}

type Service struct {
	repo repository.Repo
}

func NewService(repo repository.Repo) Servicer {
	return &Service{repo: repo}
}

func (s *Service) SaveSession(c context.Context, id int, ip, device string) error {
	return s.repo.SaveSession(c, id, ip, device)
}

func (s *Service) Login(c context.Context, pass, email string) (int, error) {
	return s.repo.Login(c, pass, email)
}

func (s *Service) CreateUser(c context.Context, user *ent.UserInput) error {
	return s.repo.CreateUser(c, user)
}

func (s *Service) GetProfessions(id int) ([]string, error) {
	return s.repo.GetProfessions(id)
}

func (s *Service) GetDataCV(item string) (*ent.CV, error) {
	return s.repo.GetDataCV(item)
}

func (s *Service) AddNewCV(cv *ent.CV) error {
	return s.repo.AddNewCV(cv)
}
