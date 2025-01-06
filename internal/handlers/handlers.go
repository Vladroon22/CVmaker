package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Vladroon22/CVmaker/internal/auth"
	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/Vladroon22/CVmaker/internal/ut"
	golog "github.com/Vladroon22/GoLog"
	"github.com/signintech/gopdf"
)

type PageCV struct {
	CV *service.CV
}

type PageData struct {
	Error error
}

type Handlers struct {
	red  *database.Redis
	logg *golog.Logger
	repo *database.Repo
	srv  *service.Service
	cvs  []service.CV
	cash map[int]*service.CV
}

func NewHandler(l *golog.Logger, r *database.Repo, s *service.Service, rd *database.Redis) *Handlers {
	return &Handlers{
		logg: l,
		repo: r,
		srv:  s,
		red:  rd,
		cvs:  make([]service.CV, 0),
		cash: make(map[int]*service.CV),
	}
}

var tmpl = template.Must(template.ParseFiles(
	filepath.Join("web", "index.html"),
	filepath.Join("web", "cv.html"),
))

func viewHandler(w http.ResponseWriter, filename string, p any) {
	err := tmpl.ExecuteTemplate(w, filename, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) HomePage(w http.ResponseWriter, r *http.Request) {
	data := PageData{}
	viewHandler(w, "index.html", data)
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self';")

	if err := r.ParseForm(); err != nil {
		maxBytes := &http.MaxBytesError{}
		if err == maxBytes {
			http.Error(w, "Request body too large (max 10 MB)", http.StatusRequestEntityTooLarge)
			h.logg.Errorln("Request body too large (max 10 MB)")
			return
		}
		http.Error(w, "Wrong input of data", http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}

	user := h.srv.UserInput
	user.Name = r.FormValue("username")
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := ut.Valid(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}

	if err := h.repo.CreateUser(&user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self';")
	user := h.srv.UserInput

	if err := r.ParseForm(); err != nil {
		maxBytes := &http.MaxBytesError{}
		if err == maxBytes {
			http.Error(w, "Request body too large (max 10 MB)", http.StatusRequestEntityTooLarge)
			h.logg.Errorln("Request body too large (max 10 MB)")
			return
		}
		http.Error(w, "Wrong input of data", http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}

	device := func(agent string) string {
		if strings.Contains(agent, "mobile") {
			return "Mobile"
		} else if strings.Contains(agent, "tablet") {
			return "Tablet"
		} else {
			return "Desktop"
		}
	}(r.Header.Get("User-Agent"))

	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := ut.Valid(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}

	id, err := h.repo.Login(user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		h.logg.Errorln(err)
		return
	}

	rt, err := auth.GenerateRT()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	if err := h.repo.SaveRT(id, rt, device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	token, err := auth.GenerateJWT(id)
	if err != nil {
		http.Error(w, "Error of creating token-session", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	SetCookie(w, "JWT", token, ut.TTLofJWT)
	SetCookie(w, "RT", rt, ut.TTLofRT)
	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
}

func (h *Handlers) parseCVForm(r *http.Request) (*service.CV, error) {
	id, err := getUserSession(r)
	if err != nil {
		h.logg.Errorln(err)
		return nil, errors.New(err.Error())
	}

	cv := &h.srv.CV

	age := r.FormValue("age")
	if !ut.ValidateDataAge(age) {
		h.logg.Errorln("Not valid data of birth")
		return nil, errors.New("not valid data of birth")
	}

	PhoneNumber := r.FormValue("phone")
	if ok := ut.ValidatePhone(PhoneNumber); !ok {
		h.logg.Errorln("Wrong phone number input in CV")
		return nil, errors.New("wrong phone number input in CV")
	}

	email := r.FormValue("emailcv")
	if ok := ut.ValidateEmail(email); !ok {
		h.logg.Errorln("Wrong email input in CV")
		return nil, errors.New("wrong email input in CV")
	}

	parts := strings.Split(age, ".")
	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year, _ := strconv.Atoi(parts[2])
	tm := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	salary := r.FormValue("salary")
	salaryInt, err := strconv.Atoi(salary)
	if err != nil {
		h.logg.Errorln(err)
		return nil, errors.New(err.Error())
	}

	cv.Age = ut.CountUserAge(tm)
	cv.Profession = r.FormValue("profession")
	cv.Name = r.FormValue("name")
	cv.Surname = r.FormValue("surname")
	cv.LivingCity = r.FormValue("city")
	cv.Education = r.FormValue("education")
	cv.Skills = r.Form["skills"]
	cv.EmailCV = email
	cv.Salary = salaryInt
	cv.PhoneNumber = PhoneNumber
	cv.ID = id

	return cv, nil
}

func (h *Handlers) MakeCV(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self';")

	if err := r.ParseForm(); err != nil {
		maxBytes := &http.MaxBytesError{}
		if err == maxBytes {
			http.Error(w, "Request body too large (max 10 MB)", http.StatusRequestEntityTooLarge)
			h.logg.Errorln("Request body too large (max 10 MB)")
			return
		}
		http.Error(w, "Wrong input of data", http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}
	parsedCV, err := h.parseCVForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.repo.AddNewCV(parsedCV); err != nil {
		http.Error(w, "Error of adding CV's data", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	h.cash[parsedCV.ID] = parsedCV
	http.Redirect(w, r, "/user/listCV", http.StatusMovedPermanently)
}

func (h *Handlers) ListCV(w http.ResponseWriter, r *http.Request) {
	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		h.logg.Errorln(err)
		return
	}

	h.cvs = []service.CV{}

	Profs, err := h.repo.GetProfessions(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	if len(Profs) == 0 {
		h.logg.Infoln("No CVs in redis")
		renderTemplate(w, "./web/cv-list.html", h.cvs)
		return
	}

	if len(Profs) == len(h.cvs) {
		h.logg.Infoln("No new CVs")
		renderTemplate(w, "./web/cv-list.html", h.cvs)
		return
	}

	for _, pr := range Profs {
		if cashCV, ok := h.cash[id]; ok {
			h.logg.Infoln("CV got from cash")
			h.checkCVInTemplates(id, *cashCV)
		} else if !ok {
			cv, err := h.repo.GetDataCV(pr)
			if err != nil {
				h.logg.Errorln("Error: ", err, " fetching CV: ", pr)
				continue
			}
			h.cash[id] = cv
			h.checkCVInTemplates(id, *cv)
			h.logg.Infoln("CV got from redis")
		}
	}

	renderTemplate(w, "./web/cv-list.html", h.cvs)
}

func (h *Handlers) checkCVInTemplates(id int, cv service.CV) {
	for _, cv := range h.cvs { // bin search?
		if cv.ID == id {
			return
		}
	}
	h.cvs = append(h.cvs, cv)
}

func renderTemplate(w http.ResponseWriter, templateFile string, data interface{}) {
	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func (h *Handlers) UserCV(w http.ResponseWriter, r *http.Request) {
	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		h.logg.Errorln("Profession not provided")
		return
	}

	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		h.logg.Errorln(err)
		return
	}

	searchCV := &h.srv.CV
	if cashCV, ok := h.cash[id]; ok {
		if cashCV.Profession == prof {
			searchCV = cashCV
		}
	} else {
		for _, cv := range h.cvs { // binsearch?
			if cv.Profession == prof {
				searchCV = &cv
				break
			}
		}
	}
	newSlice := []string{}
	for _, sk := range searchCV.Skills {
		newSlice = append(newSlice, strings.Fields(sk)...)
	}
	searchCV.Skills = newSlice
	viewHandler(w, "cv.html", searchCV)
}

func (h *Handlers) LogOut(w http.ResponseWriter, r *http.Request) {
	ClearCookie(w, "JWT")
	ClearCookie(w, "RT")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) DeleteCV(w http.ResponseWriter, r *http.Request) {
	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		h.logg.Errorln("Profession not provided")
		return
	}

	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		h.logg.Errorln(err)
		return
	}
	h.logg.Infoln("prof: " + prof)

	for i, cv := range h.cvs {
		if cv.ID == id {
			if _, ok := h.cash[id]; ok {
				delete(h.cash, id)
				h.logg.Infoln("deleted ", i, " element from cash")
			}
			if i == 0 && len(h.cvs) == 1 {
				h.cvs = h.cvs[i+1:]
				h.red.Make("lrem", "jobs", i, prof)
				h.logg.Infoln("deleted last element from redis")
				break
			} else {
				h.cvs = append(h.cvs[:i], h.cvs[i+1:]...)
				h.red.Make("lset", "jobs", i, prof)
				h.red.Make("lrem", "jobs", i, prof)
				h.logg.Infoln("deleted ", i, " element from redis")
				break
			}
		}
	}

	h.logg.Infoln(h.cvs)
	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
	tmpl, err := template.ParseFiles("./web/cv-list.html")
	tmpl.Execute(w, h.cvs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
}

func (h *Handlers) AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieJWT, err := r.Cookie("JWT")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			h.logg.Errorln(err)
			return
		}
		cookieRT, err := r.Cookie("RT")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			h.logg.Errorln(err)
		}
		claims, err := auth.ValidateJWT(cookieJWT.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			h.logg.Errorln(err)
			return
		}
		if err := h.repo.GetRT(claims.UserID, cookieRT.Value); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			h.logg.Errorln(err)
			return
		}
		if err := h.repo.CheckExpiredRT(time.Now()); err == ut.ErrTtlExceeded {
			h.LogOut(w, r)
		}
		ctx := context.WithValue(r.Context(), "id", claims.UserID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// not used yet
/*
func (h *Handlers) EditCV(w http.ResponseWriter, r *http.Request) {
	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		h.logg.Errorln("Profession not provided")
		return
	}
	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		h.logg.Errorln(err)
		return
	}

	searchCV := &h.srv.CV
	if cashCV, ok := h.cash[id]; ok {
		if cashCV.ID == id {
			searchCV = cashCV
		}
	} else {
		for _, cv := range h.cvs {
			if cv.Profession == prof {
				searchCV = &cv
				break
			}
		}
	}
	newSlice := []string{}
	for _, sk := range searchCV.Skills {
		newSlice = append(newSlice, strings.Fields(sk)...)
	}
	searchCV.Skills = newSlice
	viewHandler(w, "cv-edit.html", searchCV)
}
*/
func (h *Handlers) DownLoadPDF(w http.ResponseWriter, r *http.Request) {
	profession := r.URL.Query().Get("profession")
	h.logg.Infoln(profession)
	if profession == "" {
		http.Error(w, "profession not provided", http.StatusBadRequest)
		h.logg.Errorln("profession not provided")
		return
	}

	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		h.logg.Errorln(err)
		return
	}

	cv := h.srv.CV
	if cashCV, ok := h.cash[id]; ok {
		cv = *cashCV
	} else {
		data, err := h.red.GetData(profession)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			h.logg.Errorln(err)
			return
		}
		if err := json.Unmarshal([]byte(data), &cv); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			h.logg.Errorln(err)
			return
		}
	}

	name := cv.Name
	age := cv.Age
	prof := cv.Profession
	livingCity := cv.LivingCity
	salary := cv.Salary
	email := cv.EmailCV
	phone := cv.PhoneNumber
	education := cv.Education
	skills := cv.Skills

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	if err := pdf.AddTTFFont("LiberationSans-Bold", "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf"); err != nil {
		http.Error(w, "Error of Add TTF Font", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	if err := pdf.SetFont("LiberationSans-Bold", "", 12); err != nil {
		http.Error(w, "Error of setting font", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	yPos := 20
	lineHeight := 40

	addTitle := func(text string) {
		pdf.SetFont("LiberationSans-Bold", "", 16)
		pdf.SetX(270)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, text)
		yPos += lineHeight + 10
	}

	addText := func(label, value string) {
		pdf.SetFont("LiberationSans-Bold", "", 12)
		pdf.SetX(20)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, label+": "+value)
		yPos += lineHeight
	}

	addTitle(name)
	addText("Age", strconv.Itoa(age))
	addText("Profession", prof)
	addText("Living City", livingCity)
	addText("Salary Expectation", strconv.Itoa(salary))
	addText("Email", email)
	addText("Phone", phone)
	addText("Education", education)

	pdf.SetFont("LiberationSans-Bold", "", 12)
	pdf.SetX(float64(20))
	pdf.SetY(float64(yPos))
	pdf.Cell(nil, "Skills:")
	yPos += 15

	newSlice := []string{}
	for _, sk := range skills {
		newSlice = append(newSlice, strings.Fields(sk)...)
	}

	for _, skill := range newSlice {
		pdf.SetX(30)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "- "+skill)
		yPos += 15
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=CV.pdf")

	if _, err := pdf.WriteTo(w); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		h.logg.Errorln("Error writing PDF to response:", err)
		return
	}

	h.logg.Infoln("PDF is successfully created: CV.pdf")
}

func getUserSession(r *http.Request) (int, error) {
	token, err := r.Cookie("JWT")
	if token.Value == "" {
		return 0, errors.New("jwt is empty")
	}
	if err != nil {
		return 0, errors.New(err.Error())
	}

	claims, err := auth.ValidateJWT(token.Value)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

func SetCookie(w http.ResponseWriter, cookieName string, cookies string, ttl time.Duration) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    cookies,
		Path:     "/",
		Secure:   false, // https: true
		HttpOnly: true,
		Expires:  time.Now().Add(ttl),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

func ClearCookie(w http.ResponseWriter, cookieName string) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		Secure:   false,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}
