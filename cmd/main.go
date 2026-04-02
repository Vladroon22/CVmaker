package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/handlers"
	"github.com/Vladroon22/CVmaker/internal/repository"
	"github.com/Vladroon22/CVmaker/internal/service"
	tlsserver "github.com/Vladroon22/CVmaker/internal/tls-server"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalln(err)
	}

	db := database.NewDB()
	if err := db.Connect(context.Background()); err != nil {
		log.Fatalln(err)
	}
	redis := database.NewRedis()

	repo := repository.NewRepo(db, redis)
	srv := service.NewService(repo)
	h := handlers.NewHandler(srv)

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
	sub.HandleFunc("/downloadCV", h.DownloadPDF).Methods("GET")

	serv := tlsserver.New()

	go serv.Run(router)

	select {
	case err := <-serv.StopServerChan:
		log.Fatalf("Server error: %v", err)
		close(serv.StopServerChan)
	case sig := <-serv.StopOSChan:
		log.Printf("Received %s, shutting down...\n", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		db.Close()

		if err := serv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}

	log.Println("Gracefull shutdown")
}
