package service

import (
	"context"

	ent "github.com/Vladroon22/CVmaker/internal/entity"
)

type Servicer interface {
	SaveSession(context.Context, int, string) error
	Login(context.Context, string, string) (int, error)
	CreateUser(context.Context, *ent.UserInput) error
	GetProfessions(int) ([]string, error)
	GetDataCV(int, string) (*ent.CV, error)
	AddNewCV(*ent.CV) error
	DeleteCV(int, string) error
}

type Service struct {
	repo Servicer
}

func NewService(repo Servicer) Servicer {
	return &Service{repo: repo}
}

func (s *Service) SaveSession(c context.Context, id int, device string) error {
	return s.repo.SaveSession(c, id, device)
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

func (s *Service) GetDataCV(id int, item string) (*ent.CV, error) {
	return s.repo.GetDataCV(id, item)
}

func (s *Service) AddNewCV(cv *ent.CV) error {
	return s.repo.AddNewCV(cv)
}

func (s *Service) DeleteCV(id int, item string) error {
	return s.repo.DeleteCV(id, item)
}
