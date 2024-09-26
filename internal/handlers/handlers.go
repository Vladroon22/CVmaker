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

type PageData struct {
	Error string
}

type Handlers struct {
	R    *mux.Router
	logg *golog.Logger
	repo *database.Repo
	srv  *service.Service
}

func NewHandler(r *database.Repo, h *mux.Router, l *golog.Logger, s *service.Service) *Handlers {
	return &Handlers{
		repo: r,
		R:    h,
		logg: l,
		srv:  s,
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
	user := h.srv.UserInput
	user.Name = r.FormValue("username")
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	id, err := h.repo.CreateUser(user.Name, user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"id": id,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	user := h.srv.UserInput
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	token, err := h.repo.GenerateJWT(user.Password, user.Email)
	if err != nil {
		h.logg.Errorln(err)
		return
	}
	if token == "" {
		http.Error(w, "token is empty", http.StatusUnauthorized)
		h.logg.Errorln("token is empty")
		return
	}

	setCookie(w, "jwt", token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) MakeCV(w http.ResponseWriter, r *http.Request) {
	cv := h.srv.CV
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	salary := r.FormValue("salary")
	cv.Name = r.FormValue("name")
	cv.Surname = r.FormValue("surname")
	cv.PhoneNumber = r.FormValue("phone")
	cv.LivingCity = r.FormValue("city")
	cv.EmailCV = r.FormValue("emailcv")
	cv.Salary, _ = strconv.Atoi(salary)
	cv.Education = r.FormValue("education")
	cv.Skills = r.Form["skills"]

	id, err := h.repo.AddNewCV(cv.Salary, cv.Name, cv.Surname, cv.EmailCV, cv.LivingCity, cv.PhoneNumber, cv.Education)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	errTx := h.repo.InsertSkills(id, cv.Skills...)
	if errTx != nil {
		http.Error(w, errTx.Error(), http.StatusInternalServerError)
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
