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

	srv := service.NewService()
	repo := database.NewRepo(db, cnf, srv)
	r := handlers.NewHandler(repo, mux.NewRouter(), logger, srv)

	r.R.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web"))))

	r.R.HandleFunc("/", r.HomePage).Methods("GET")
	r.R.HandleFunc("/sign-up", r.Register).Methods("GET", "POST")
	r.R.HandleFunc("/sign-in", handlers.AuthMiddleWare(r.SignIn)).Methods("GET", "POST")
	r.R.HandleFunc("/makeCV", r.MakeCV).Methods("GET", "POST")
	r.R.HandleFunc("/profile", r.UserCV).Methods("GET")
	r.R.HandleFunc("/listCV", r.ListCV).Methods("GET")
	r.R.HandleFunc("/logout", r.LogOut).Methods("GET")

	go http.ListenAndServe(cnf.Addr_PORT, r.R)
	logger.Infoln("Server is listening --> localhost" + cnf.Addr_PORT)

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
