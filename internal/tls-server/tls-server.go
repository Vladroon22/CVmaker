package tlsserver

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	golog "github.com/Vladroon22/GoLog"
	"github.com/gorilla/mux"
)

type Server struct {
	logger         *golog.Logger
	server         *http.Server
	StopOSChan     chan os.Signal
	StopServerChan chan error
}

func New(log *golog.Logger) *Server {
	return &Server{
		server:         &http.Server{},
		logger:         log,
		StopOSChan:     make(chan os.Signal, 1),
		StopServerChan: make(chan error, 1),
	}
}

func (s *Server) Run(router *mux.Router) error {
	signal.Notify(s.StopOSChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	certFile := os.Getenv("cert")
	keyFile := os.Getenv("keys")

	_, errCert := os.Stat(certFile)
	_, errKey := os.Stat(keyFile)

	addr := os.Getenv("addr")

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
		s.StopServerChan <- err
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
