package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/service"
	golog "github.com/Vladroon22/GoLog"
)

const (
	TTL = time.Hour
)

type PageCV struct {
	CV *service.CV
}

type PageUsersCV struct {
	Profession string
}

type PageData struct {
	Error error
}

type Handlers struct {
	red  *database.Redis
	logg *golog.Logger
	repo *database.Repo
	srv  *service.Service
	data []PageUsersCV
	cvs  []service.CV
}

func NewHandler(l *golog.Logger, r *database.Repo, s *service.Service, rd *database.Redis) *Handlers {

	return &Handlers{
		logg: l,
		repo: r,
		srv:  s,
		red:  rd,
		data: make([]PageUsersCV, 0),
		cvs:  make([]service.CV, 0),
	}
}

func (h *Handlers) HomePage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("./web/index.html")
	data := PageData{}
	tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	user := h.srv.UserInput
	user.Name = r.FormValue("username")
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := h.repo.CreateUser(&user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	user := h.srv.UserInput

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	id, err := h.repo.Login(user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	token, err := h.repo.GenerateJWT(id, user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		h.logg.Errorln(err)
		return
	}

	if err := h.red.SetData("JWT", token, time.Hour); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
}

func (h *Handlers) MakeCV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	cv := &h.srv.CV
	salary := r.FormValue("salary")
	age := r.FormValue("age")
	cv.Profession = r.FormValue("profession")
	cv.Age, _ = strconv.Atoi(age)
	cv.Name = r.FormValue("name")
	cv.Surname = r.FormValue("surname")
	cv.PhoneNumber = r.FormValue("phone")
	cv.LivingCity = r.FormValue("city")
	cv.EmailCV = r.FormValue("emailcv")
	cv.Salary, _ = strconv.Atoi(salary)
	cv.Education = r.FormValue("education")
	cv.Skills = r.Form["skills"]

	err := h.repo.AddNewCV(cv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	h.data = append(h.data, PageUsersCV{Profession: cv.Profession})

	http.Redirect(w, r, "/user/listCV", http.StatusMovedPermanently)
}

func (h *Handlers) ListCV(w http.ResponseWriter, r *http.Request) {
	Profs, err := h.repo.GetProfessionCV()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	for _, pr := range Profs {
		if len(Profs) == len(h.data) {
			break
		}
		cv, err := h.repo.GetDataCV(pr)
		if err != nil {
			h.logg.Errorln("Error fetching CV for profession: ", err)
			h.logg.Errorln(pr)
			continue
		}
		h.data = append(h.data, PageUsersCV{Profession: pr})
		h.cvs = append(h.cvs, cv)
	}

	tmpl, err := template.ParseFiles("./web/cv-list.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	err = tmpl.Execute(w, h.data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
}

func (h *Handlers) UserCV(w http.ResponseWriter, r *http.Request) {
	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		h.logg.Errorln("Profession not provided")
		return
	}

	searchCV := &h.srv.CV
	for _, cv := range h.cvs {
		if cv.Profession == prof {
			searchCV = &cv
			break
		}
	}
	newSlice := []string{}
	for _, sk := range searchCV.Skills {
		newSlice = append(newSlice, strings.Fields(sk)...)
	}
	searchCV.Skills = newSlice
	tmpl, err := template.ParseFiles("./web/cv.html")
	tmpl.Execute(w, searchCV)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
}

func (h *Handlers) LogOut(w http.ResponseWriter, r *http.Request) {
	h.red.Make("del", "JWT")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := h.red.GetData("JWT")
		if err != nil || token == "" {
			http.Error(w, "JWT not exists", http.StatusUnauthorized)
			h.logg.Errorln("JWT not exists")
			return
		}
		claims, err := database.ValidateToken(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			h.logg.Errorln(err)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), "id", claims.UserID))

		next.ServeHTTP(w, r)
	})
}
