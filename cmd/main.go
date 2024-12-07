package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Vladroon22/CVmaker/config"
	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/handlers"
	"github.com/Vladroon22/CVmaker/internal/service"
	tlsserver "github.com/Vladroon22/CVmaker/internal/tls-server"
	golog "github.com/Vladroon22/GoLog"
	"github.com/gorilla/mux"
)

func main() {
	cnf := config.CreateConfig()
	logger := golog.New()

	db := database.NewDB(cnf, logger)
	if err := db.Connect(); err != nil {
		logger.Fatalln(err)
	}

	redis := database.NewRedis(logger)

	router := mux.NewRouter()
	srv := service.NewService()
	repo := database.NewRepo(db, srv, redis)
	h := handlers.NewHandler(logger, repo, srv, redis)

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))

	router.HandleFunc("/", h.HomePage).Methods("GET")
	router.HandleFunc("/sign-up", h.Register).Methods("POST")
	router.HandleFunc("/sign-in", h.SignIn).Methods("POST")
	router.HandleFunc("/logout", h.LogOut).Methods("GET")

	sub := router.PathPrefix("/user/").Subrouter()
	sub.Use(h.AuthMiddleWare)

	sub.HandleFunc("/deleteCV", h.DeleteCV).Methods("GET")
	sub.HandleFunc("/makeCV", h.MakeCV).Methods("PUT", "POST")
	sub.HandleFunc("/profile", h.UserCV).Methods("GET")
	sub.HandleFunc("/listCV", h.ListCV).Methods("GET")
	sub.HandleFunc("/editCV", h.EditCV).Methods("PUT", "PATCH")
	sub.HandleFunc("/downloadCV", h.DownLoadPDF).Methods("GET")

	serv := tlsserver.New(cnf, logger)
	go func() {
		if err := serv.Run(router); err != nil || err != http.ErrServerClosed {
			logger.Fatalln(err)
		}
	}()

	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, syscall.SIGINT, syscall.SIGTERM)
	<-exitSig

	go func() {
		wg := &sync.WaitGroup{}
		wg.Add(1)

		defer wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := db.CloseDB(); err != nil {
			logger.Fatalln(err)
		}

		if err := serv.Shutdown(ctx); err != nil {
			logger.Fatalln(err)
		}

		wg.Wait()
	}()

	logger.Infoln("Gracefull shutdown")
}
