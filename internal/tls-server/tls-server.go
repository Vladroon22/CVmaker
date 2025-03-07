package tlsserver

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	golog "github.com/Vladroon22/GoLog"
	"github.com/gorilla/mux"
)

type Server struct {
	logger *golog.Logger
	server *http.Server
}

func New(log *golog.Logger) *Server {
	return &Server{
		server: &http.Server{},
		logger: log,
	}
}

func (s *Server) Run(router *mux.Router) error {
	certFile := os.Getenv("cert")
	keyFile := os.Getenv("keys")

	_, errCert := os.Stat(certFile)
	_, errKey := os.Stat(keyFile)

	addr := os.Getenv("addr")
	if addr == "127.0.0.1" || addr == "0.0.0.0" {
		addr = ""
	}

	if os.IsNotExist(errCert) || os.IsNotExist(errKey) {
		s.server = &http.Server{
			Addr:           addr + ":" + os.Getenv("port"),
			Handler:        router,
			MaxHeaderBytes: 1 << 20,
			WriteTimeout:   15 * time.Second,
			ReadTimeout:    15 * time.Second,
		}
		s.logger.Infoln("Server is listening --> http://", os.Getenv("addr")+":"+os.Getenv("port"))
		return s.server.ListenAndServe()
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		s.logger.Fatalln(err)
	}

	s.server = &http.Server{
		TLSConfig:      &tls.Config{Certificates: []tls.Certificate{certificate}},
		Addr:           os.Getenv("addr") + ":" + os.Getenv("portS"),
		Handler:        router,
		MaxHeaderBytes: 1 << 20,
		WriteTimeout:   15 * time.Second,
		ReadTimeout:    15 * time.Second,
	}

	s.logger.Infoln("Server is listening --> https://", os.Getenv("addr")+":"+os.Getenv("portS"))
	return s.server.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
