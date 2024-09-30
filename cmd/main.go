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

	redis := database.NewRedis(logger)

	router := mux.NewRouter()
	srv := service.NewService()
	repo := database.NewRepo(db, srv, redis)
	h := handlers.NewHandler(logger, repo, srv)

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web"))))

	router.HandleFunc("/", h.HomePage).Methods("GET")
	router.HandleFunc("/sign-up", h.Register).Methods("POST")
	router.HandleFunc("/sign-in", h.SignIn).Methods("POST")

	sub := router.PathPrefix("/user/").Subrouter()
	h.AuthMiddleWare(sub)

	router.HandleFunc("/user/makeCV", h.MakeCV).Methods("PUT", "POST")
	router.HandleFunc("/user/profile", h.UserCV).Methods("GET")
	router.HandleFunc("/user/listCV", h.ListCV).Methods("GET")
	router.HandleFunc("/user/logout", h.LogOut).Methods("GET")

	logger.Infoln("Server is listening --> localhost" + cnf.Addr_PORT)
	go http.ListenAndServe(cnf.Addr_PORT, router)

	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, syscall.SIGINT, syscall.SIGTERM)
	<-exitSig

	go func() {
		if err := db.CloseDB(); err != nil {
			logger.Fatalln(err)
			return
		}
	}()

	logger.Infoln("Gracefull shutdown")
}
