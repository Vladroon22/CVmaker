package tlsserver

import (
	"context"
	"crypto/tls"
	"errors"
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
	certFile := ""
	keyFile := ""

	cFile, err := os.Stat("cert.crt")
	if err != nil {
		return err
	}

	kFile, err := os.Stat("Key.key")
	if err != nil {
		return err
	}

	certFile = cFile.Name()
	keyFile = kFile.Name()
	if certFile != "" && keyFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			s.logger.Fatalln(err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{certificate},
		}

		s.server = &http.Server{
			TLSConfig:      tlsConfig,
			Addr:           ":" + s.conf.Addr_PORT,
			Handler:        router,
			MaxHeaderBytes: 1 << 20,
			WriteTimeout:   15 * time.Second,
			ReadTimeout:    15 * time.Second,
		}

		s.logger.Infoln("Server is listening --> https://localhost", ":"+s.conf.Addr_PORT)
		return s.server.ListenAndServeTLS(certFile, keyFile)
	} else if certFile == "" && keyFile == "" {
		s.server = &http.Server{
			Addr:           ":" + s.conf.Addr_PORT,
			Handler:        router,
			MaxHeaderBytes: 1 << 20,
			WriteTimeout:   15 * time.Second,
			ReadTimeout:    15 * time.Second,
		}
		s.logger.Infoln("Server is listening --> localhost", ":"+s.conf.Addr_PORT)
		return s.server.ListenAndServe()
	} else {
		return errors.New("Invalid-cert-or-key")
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
