package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/handlers"
	"github.com/Vladroon22/CVmaker/internal/repository"
	"github.com/Vladroon22/CVmaker/internal/service"
	tlsserver "github.com/Vladroon22/CVmaker/internal/tls-server"
	golog "github.com/Vladroon22/GoLog"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	logger := golog.New()

	if err := godotenv.Load(); err != nil {
		logger.Fatalln(err)
	}

	file, err := logger.SetOutput("logs.txt")
	if err != nil {
		logger.Fatalln(err)
	}
	defer file.Close()

	conn, err := database.NewDB(logger).Connect(context.Background())
	if err != nil {
		logger.Fatalln(err)
	}
	redis := database.NewRedis(logger)

	repo := repository.NewRepo(conn, logger, redis)
	srv := service.NewService(repo)
	h := handlers.NewHandler(logger, srv)

	router := mux.NewRouter()
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))

	router.HandleFunc("/", h.HomePage).Methods("GET")
	router.HandleFunc("/sign-up", h.Register).Methods("POST")
	router.HandleFunc("/sign-in", h.SignIn).Methods("POST")
	router.HandleFunc("/logout", h.LogOut).Methods("GET")

	sub := router.PathPrefix("/user/").Subrouter()
	sub.Use(h.AuthMiddleWare)

	sub.HandleFunc("/deleteCV", h.DeleteCV).Methods("GET")
	sub.HandleFunc("/makeCV", h.MakeCV).Methods("POST")
	sub.HandleFunc("/profile", h.UserCV).Methods("GET")
	sub.HandleFunc("/listCV", h.ListCV).Methods("GET")
	sub.HandleFunc("/downloadCV", h.DownLoadPDF).Methods("GET")

	serv := tlsserver.New(logger)
	go func() {
		if err := serv.Run(router); err != nil && err != http.ErrServerClosed {
			logger.Fatalln(err)
		}
	}()

	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, syscall.SIGINT, syscall.SIGTERM)
	<-exitSig

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := serv.Shutdown(ctx); err != nil {
			logger.Errorln(err)
		}

		conn.Close()
	}()
	wg.Wait()

	logger.Infoln("Gracefull shutdown")
}
