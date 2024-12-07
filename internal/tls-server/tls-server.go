package tlsserver

import (
	"context"
	"crypto/tls"
	"flag"
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
	var cert, key string
	flag.StringVar(&cert, "cert", "", "Path to the certificate file")
	flag.StringVar(&key, "key", "", "Path to the private key file")
	flag.Parse()
	if cert != "" && key != "" {
		certificate, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			s.logger.Fatalln(err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{certificate},
		}

		s.server = &http.Server{
			TLSConfig:      tlsConfig,
			Addr:           s.conf.Addr_PORT,
			Handler:        router,
			MaxHeaderBytes: 1 << 20,
			WriteTimeout:   15 * time.Second,
			ReadTimeout:    15 * time.Second,
		}

		s.logger.Infoln("Server is listening --> https://localhost", s.conf.Addr_PORT)
		return s.server.ListenAndServeTLS(cert, key)
	} else if cert == "" && key == "" {
		s.server = &http.Server{
			Addr:           s.conf.Addr_PORT,
			Handler:        router,
			MaxHeaderBytes: 1 << 20,
			WriteTimeout:   15 * time.Second,
			ReadTimeout:    15 * time.Second,
		}
		s.logger.Infoln("Server is listening --> localhost", s.conf.Addr_PORT)
		return s.server.ListenAndServe()
	} else {
		return http.ErrServerClosed
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
