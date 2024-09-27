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
	"github.com/gorilla/mux"
)

const (
	TTL = time.Hour
)

type PageUsersCV struct {
	ID         int
	Profession string
}

type PageData struct {
	Error string
}

type Handlers struct {
	R    *mux.Router
	logg *golog.Logger
	repo *database.Repo
	srv  *service.Service
	data []PageUsersCV
}

func NewHandler(r *database.Repo, h *mux.Router, l *golog.Logger, s *service.Service) *Handlers {
	return &Handlers{
		repo: r,
		R:    h,
		logg: l,
		srv:  s,
		data: make([]PageUsersCV, 10),
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
	tmpl, err := template.ParseFiles("./web/sign-up.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, PageData{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	user := h.srv.UserInput
	user.Name = r.FormValue("username")
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := h.repo.CreateUser(user.Name, user.Password, user.Email); err != nil {
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

	token, err := h.repo.GenerateJWT(user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		h.logg.Errorln(err)
		return
	}

	setCookie(w, "jwt", token)
	http.Redirect(w, r, "/listCV", http.StatusSeeOther)
}

func (h *Handlers) MakeCV(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("./web/makecv.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, PageData{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cv := h.srv.CV
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
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

	id, err := h.repo.AddNewCV(cv.Age, cv.Salary, cv.Profession, cv.Name, cv.Surname, cv.EmailCV, cv.LivingCity, cv.PhoneNumber, cv.Education, cv.Skills...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	h.data = append(h.data, PageUsersCV{ID: id, Profession: cv.Profession})
}

func (h *Handlers) ListCV(w http.ResponseWriter, r *http.Request) {
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

	dataCV, err := h.repo.GetDataCV(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	tmpl, err := template.ParseFiles("./web/cv.html")
	tmpl.Execute(w, dataCV)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
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

func AuthMiddleWare(next http.HandlerFunc) http.HandlerFunc {
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
		r = r.WithContext(context.WithValue(r.Context(), "id", claims.UserId))

		next(w, r)
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
