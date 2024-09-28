package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/service"
	golog "github.com/Vladroon22/GoLog"
	"github.com/dgrijalva/jwt-go"
)

const (
	TTL = time.Hour
)

type PageCV struct {
	CV *service.CV
}

type PageUsersCV struct {
	ID         int
	Profession string
}

type PageData struct {
	Error error
}

type Handlers struct {
	logg *golog.Logger
	repo *database.Repo
	srv  *service.Service
	data []PageUsersCV // need to store in cash (redis)
	cvs  []service.CV  // need to store in cash (redis)
}

func NewHandler(l *golog.Logger, r *database.Repo, s *service.Service) *Handlers {
	return &Handlers{
		logg: l,
		repo: r,
		srv:  s,
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
	setCookie(w, "jwt", token)
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

	id, err := h.repo.AddNewCV(cv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	h.data = append(h.data, PageUsersCV{ID: id, Profession: cv.Profession})
	tmpl, err := template.ParseFiles("./web/cv-list.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, h.data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) ListCV(w http.ResponseWriter, r *http.Request) {
	dataCV, err := h.repo.CheckDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, cv := range dataCV {
		h.data = append(h.data, PageUsersCV{ID: cv.ID, Profession: cv.Profession})
		h.cvs = append(h.cvs, *cv)
	}
	tmpl, err := template.ParseFiles("./web/cv-list.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, h.data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) UserCV(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "ID not provided", http.StatusBadRequest)
		h.logg.Errorln("ID not provided")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}
	searchCV := &service.CV{}
	for _, cv := range h.cvs {
		if cv.ID == id {
			searchCV = &cv
		}
	}
	tmpl, err := template.ParseFiles("./web/cv.html")
	tmpl.Execute(w, searchCV)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
}

func (h *Handlers) LogOut(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, "jwt", "")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func setCookie(w http.ResponseWriter, cookieName string, cookies string) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    cookies,
		Path:     "/",
		Secure:   false,
		HttpOnly: true,
		Expires:  time.Now().Add(TTL),
	}
	http.SetCookie(w, cookie)
}

func clearCookie(w http.ResponseWriter, cookieName string, cookies string) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    cookies,
		Path:     "/",
		Expires:  time.Unix(0, 0),
		Secure:   false,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

func (h *Handlers) AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("jwt")
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if cookie.Value == "" {
			http.Error(w, "Cookie is empty", http.StatusUnauthorized)
			return
		}
		claims, err := validateToken(cookie.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), "id", claims.UserID))

		next.ServeHTTP(w, r)
	})
}

func validateToken(tokenStr string) (*database.MyClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &database.MyClaims{}, func(token *jwt.Token) (interface{}, error) {
		return database.SignKey, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			return nil, errors.New("Unauthorized")
		}
		return nil, errors.New("Bad-Request")
	}

	claims, ok := token.Claims.(*database.MyClaims)
	if !ok || !token.Valid {
		return nil, errors.New("Unauthorized")
	}

	return claims, nil
}

func WriteJSON(w http.ResponseWriter, status int, a interface{}) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(a)
}
