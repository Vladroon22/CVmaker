package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Vladroon22/CVmaker/config"
	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/handlers"
	"github.com/Vladroon22/CVmaker/internal/service"
	golog "github.com/Vladroon22/GoLog"
	"github.com/gorilla/mux"
)

func main() {
	cnf := config.CreateConfig()
	logger := golog.New()

	db := database.NewDB(cnf, logger)
	if err := db.Connect(); err != nil {
		logger.Fatalln(err)
		return
	}

	router := mux.NewRouter()
	srv := service.NewService()
	repo := database.NewRepo(db, cnf, srv)
	r := handlers.NewHandler(repo, router, logger, srv)

	router.HandleFunc("/", r.HomePage).Methods("GET")
	router.HandleFunc("/sign-up", r.Register).Methods("POST")
	router.HandleFunc("/sign-in", handlers.AuthMiddleWare(r.SignIn)).Methods("POST")
	router.HandleFunc("/makeCV", r.MakeCV).Methods("POST")
	router.HandleFunc("/logout", r.LogOut).Methods("GET")

	go http.ListenAndServe(cnf.Addr_PORT, router)
	logger.Infoln("Server is listening -->" + cnf.Addr_PORT)

	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, syscall.SIGINT, syscall.SIGTERM)
	<-exitSig

	go func() {
		if err := db.CloseDB(); err != nil {
			logger.Fatalln(err)
			return
		}
	}()
}
