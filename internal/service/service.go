package service

import (
	"context"

	ent "github.com/Vladroon22/CVmaker/internal/entity"
)

type Servicer interface {
	SaveSession(context.Context, string, string) error
	Login(context.Context, string, string) (string, error)
	CreateUser(context.Context, *ent.UserInput) error
	GetProfessions(string) ([]string, error)
	GetDataCV(string, string) (*ent.CV, error)
	AddNewCV(*ent.CV) error
	DeleteCV(string, string) error
}

type Service struct {
	repo Servicer
}

func NewService(repo Servicer) Servicer {
	return &Service{repo: repo}
}

func (s *Service) SaveSession(c context.Context, id string, device string) error {
	return s.repo.SaveSession(c, id, device)
}

func (s *Service) Login(c context.Context, pass, email string) (string, error) {
	return s.repo.Login(c, pass, email)
}

func (s *Service) CreateUser(c context.Context, user *ent.UserInput) error {
	return s.repo.CreateUser(c, user)
}

func (s *Service) GetProfessions(id string) ([]string, error) {
	return s.repo.GetProfessions(id)
}

func (s *Service) GetDataCV(id string, item string) (*ent.CV, error) {
	return s.repo.GetDataCV(id, item)
}

func (s *Service) AddNewCV(cv *ent.CV) error {
	return s.repo.AddNewCV(cv)
}

func (s *Service) DeleteCV(id, item string) error {
	return s.repo.DeleteCV(id, item)
}
