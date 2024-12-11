package tlsserver

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
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
	certFile := "cert.crt"
	keyFile := "Key.key"

	_, err1 := os.Stat("cert.crt")
	_, err2 := os.Stat("Key.key")

	if os.IsNotExist(err1) || os.IsNotExist(err2) {
		s.server = &http.Server{
			Addr:           ":" + s.conf.Addr_PORT,
			Handler:        router,
			MaxHeaderBytes: 1 << 20,
			WriteTimeout:   15 * time.Second,
			ReadTimeout:    15 * time.Second,
		}
		s.logger.Infoln("Server is listening --> localhost", ":"+s.conf.Addr_PORT)
		return s.server.ListenAndServe()
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		s.logger.Fatalln(err)
	}

	s.server = &http.Server{
		TLSConfig:      &tls.Config{Certificates: []tls.Certificate{certificate}},
		Addr:           ":" + s.conf.Addr_PORT,
		Handler:        router,
		MaxHeaderBytes: 1 << 20,
		WriteTimeout:   15 * time.Second,
		ReadTimeout:    15 * time.Second,
	}

	s.logger.Infoln("Server is listening --> https://localhost", ":"+s.conf.Addr_PORT)
	return s.server.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
