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
	}

	redis := database.NewRedis(logger)

	router := mux.NewRouter()
	srv := service.NewService()
	repo := database.NewRepo(db, srv, redis)
	h := handlers.NewHandler(logger, repo, srv, redis)

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/"))))

	router.HandleFunc("/", h.HomePage).Methods("GET")
	router.HandleFunc("/sign-up", h.Register).Methods("POST")
	router.HandleFunc("/sign-in", h.SignIn).Methods("POST")
	router.HandleFunc("/logout", h.LogOut).Methods("GET")

	sub := router.PathPrefix("/user/").Subrouter()
	sub.Use(h.AuthMiddleWare)

	sub.HandleFunc("/makeCV", h.MakeCV).Methods("PUT", "POST")
	sub.HandleFunc("/profile", h.UserCV).Methods("GET")
	sub.HandleFunc("/listCV", h.ListCV).Methods("GET")
	sub.HandleFunc("/editCV", h.EditCV).Methods("PUT", "PATCH")
	sub.HandleFunc("/downloadCV", h.DownLoadPDF).Methods("GET")
	sub.HandleFunc("/deleteCV", h.DeleteCV).Methods("GET")

	logger.Infoln("Server is listening --> localhost" + cnf.Addr_PORT)
	go http.ListenAndServe(cnf.Addr_PORT, router)

	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, syscall.SIGINT, syscall.SIGTERM)
	<-exitSig

	go func() {
		if err := db.CloseDB(); err != nil {
			logger.Fatalln(err)
		}
	}()

	logger.Infoln("Gracefull shutdown")
}
