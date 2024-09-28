package server

import (
	"context"
	"net/http"
	"time"

	"github.com/Vladroon22/CVmaker/config"
	golog "github.com/Vladroon22/GoLog"
	"github.com/gorilla/mux"
)

type Server struct {
	conf   *config.Config
	logger *golog.Logger
	server *http.Server
}

func New(conf *config.Config, log *golog.Logger) *Server {
	return &Server{
		server: &http.Server{},
		conf:   conf,
		logger: log,
	}
}

func (s *Server) Run(router *mux.Router) error {
	s.logger.Infoln("Init router")

	s.server = &http.Server{
		Addr:           s.conf.Addr_PORT,
		Handler:        router,
		MaxHeaderBytes: 1 << 20,
		WriteTimeout:   15 * time.Second,
		ReadTimeout:    15 * time.Second,
	}

	s.logger.Infoln("Server is listening -->", s.conf.Addr_PORT)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
