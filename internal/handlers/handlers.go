package handlers

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Vladroon22/CVmaker/internal/auth"
	"github.com/Vladroon22/CVmaker/internal/database"
	ent "github.com/Vladroon22/CVmaker/internal/entity"
	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/Vladroon22/CVmaker/internal/utils"
	golog "github.com/Vladroon22/GoLog"
	"github.com/signintech/gopdf"
)

type PageCV struct {
	CV *ent.CV
}

type PageData struct {
	Error error
}

type Handlers struct {
	red  *database.Redis
	logg *golog.Logger
	srv  service.Servicer
	cvs  []ent.CV
	cash map[int]*ent.CV
}

func NewHandler(l *golog.Logger, s service.Servicer, rd *database.Redis) *Handlers {
	return &Handlers{
		logg: l,
		srv:  s,
		red:  rd,
		cvs:  make([]ent.CV, 0),
		cash: make(map[int]*ent.CV),
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
	if err := h.checkValidRequest(w, r); err != nil {
		h.logg.Errorln(err)
		return
	}

	user := ent.UserInput{}
	user.Name = r.FormValue("username")
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := utils.Valid(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}

	if err := h.srv.CreateUser(r.Context(), &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	if err := h.checkValidRequest(w, r); err != nil {
		h.logg.Errorln(err)
		return
	}

	user := ent.UserInput{}
	device := getUserDevice(r.Header.Get("User-Agent"))
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := utils.Valid(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}

	id, err := h.srv.Login(r.Context(), user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		h.logg.Errorln(err)
		return
	}

	if err := h.srv.SaveSession(r.Context(), id, r.RemoteAddr, device); err != nil {
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

	setCookie(w, "JWT", token, utils.TTLofJWT)
	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
}

func (h *Handlers) parseCVForm(r *http.Request) (*ent.CV, error) {
	id, err := getUserSession(r)
	if err != nil {
		h.logg.Errorln(err)
		return nil, errors.New("CV wasn't find")
	}

	cv := &ent.CV{}

	age := r.FormValue("age")
	if !utils.ValidateDataAge(age) {
		h.logg.Errorln("Not valid data of birth")
		return nil, errors.New("not valid data of birth")
	}

	PhoneNumber := r.FormValue("phone")
	if ok := utils.ValidatePhone(PhoneNumber); !ok {
		h.logg.Errorln("Wrong phone number input in CV")
		return nil, errors.New("wrong phone number input in CV")
	}

	email := r.FormValue("emailcv")
	if ok := utils.ValidateEmail(email); !ok {
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
		return nil, errors.New("salary set wrong format")
	}

	cv.Age = utils.CountUserAge(tm)
	cv.Profession = r.FormValue("profession")
	cv.Name = r.FormValue("name")
	cv.Surname = r.FormValue("surname")
	cv.LivingCity = r.FormValue("city")
	cv.Education = r.FormValue("education")
	cv.SoftSkills = r.Form["softskills"]
	cv.HardSkills = r.Form["hardskills"]
	cv.Description = r.FormValue("description")
	cv.EmailCV = email
	cv.Salary = salaryInt
	cv.Currency = r.FormValue("currency")
	cv.PhoneNumber = PhoneNumber
	cv.ID = id

	return cv, nil
}

func (h *Handlers) MakeCV(w http.ResponseWriter, r *http.Request) {
	if err := h.checkValidRequest(w, r); err != nil {
		h.logg.Errorln(err)
		return
	}

	parsedCV, err := h.parseCVForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.srv.AddNewCV(parsedCV); err != nil {
		http.Error(w, "CV's data sent incorrectly", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
	h.cash[parsedCV.ID] = parsedCV
	http.Redirect(w, r, "/user/listCV", http.StatusMovedPermanently)
}

func (h *Handlers) ListCV(w http.ResponseWriter, r *http.Request) {
	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, "CV wasn't find", http.StatusUnauthorized)
		h.logg.Errorln(err)
		return
	}

	Profs, err := h.srv.GetProfessions(id)
	if err != nil {
		http.Error(w, "Profession's data got incorrectly", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	h.cvs = []ent.CV{}
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

	h.logg.Infoln("Professions: ", Profs)

	h.cvs = h.handleProfessions(Profs, id)

	h.logg.Infoln("CVs: ", len(h.cvs))
	renderTemplate(w, "./web/cv-list.html", h.cvs)
}

func renderTemplate(w http.ResponseWriter, templateFile string, data interface{}) {
	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		http.Error(w, "Error of presenting data", http.StatusInternalServerError)
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
		http.Error(w, "CV wasn't find", http.StatusForbidden)
		h.logg.Errorln(err)
		return
	}

	searchCV, err := h.getUserCV(id, prof)
	if err != nil {
		http.Error(w, "Error of receive data", http.StatusInternalServerError)
		h.logg.Errorln("Error of receive data from redis: ", err)
		return
	}

	if len(searchCV.SoftSkills) != 0 {
		soft := []string{}
		for _, sk := range searchCV.SoftSkills {
			soft = append(soft, strings.Fields(sk)...)
		}
		searchCV.SoftSkills = soft
	}

	if len(searchCV.HardSkills) != 0 {
		hard := []string{}
		for _, sk := range searchCV.HardSkills {
			hard = append(hard, strings.Fields(sk)...)
		}
		searchCV.HardSkills = hard
	}

	viewHandler(w, "cv.html", searchCV)
}

func (h *Handlers) LogOut(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, "JWT")
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
		http.Error(w, "CV wasn't find", http.StatusForbidden)
		h.logg.Errorln(err)
		return
	}
	h.logg.Infoln("prof: " + prof)

	i := utils.BinSearchIndex(h.cvs, id, prof)
	if i == -1 {
		http.Error(w, "no such profession", http.StatusBadRequest)
		h.logg.Errorln("index out of range (BinSearchIndex): ", i)
		return
	}

	if _, ok := h.cash[id]; ok {
		delete(h.cash, id)
		h.logg.Infoln("deleted element with from cache")
	}

	if len(h.cvs) == 1 {
		h.cvs = h.cvs[i+1:]
	} else {
		h.cvs = append(h.cvs[:i], h.cvs[i+1:]...)
	}

	h.red.Make("lrem", "jobs", i, prof)
	h.logg.Infoln("deleted element from redis at index: ", i)

	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
	renderTemplate(w, "./web/cv-list.html", h.cvs)
}

func (h *Handlers) AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ID any = "id"
		cookieJWT, err := r.Cookie("JWT")
		if err == http.ErrNoCookie {
			http.Error(w, "Session expired", http.StatusUnauthorized)
			h.logg.Errorln("Session expired: ", err)
			return
		}
		claims, err := auth.ValidateJWT(cookieJWT.Value)
		if err != nil {
			http.Error(w, "Invalid JWT", http.StatusUnauthorized)
			h.logg.Errorln("Invalid JWT: ", err)
			return
		}
		ctx := context.WithValue(r.Context(), ID, claims.UserID)
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
}
*/
func (h *Handlers) DownLoadPDF(w http.ResponseWriter, r *http.Request) {
	profession := r.URL.Query().Get("profession")
	h.logg.Infoln("Converting in pdf... ", profession)
	if profession == "" {
		http.Error(w, "profession provided", http.StatusBadRequest)
		h.logg.Errorln("profession not provided")
		return
	}

	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, "CV wasn't find", http.StatusForbidden)
		h.logg.Errorln(err)
		return
	}

	cv, err := h.getUserCV(id, profession)
	if err != nil {
		http.Error(w, "Error of receive data", http.StatusInternalServerError)
		h.logg.Errorln("Error of receive data from redis: ", err)
		return
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	if err := pdf.AddTTFFont("LiberationSans-Bold", "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf"); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	if err := pdf.SetFont("LiberationSans-Bold", "", 12); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	yPos := 20
	lineHeight := 27

	addTitle := func(text string) {
		pdf.SetFont("LiberationSans-Bold", "", 16)
		pdf.SetX(230)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, text)
		yPos += lineHeight + 15
	}

	addText := func(label, value string) {
		pdf.SetFont("LiberationSans-Bold", "", 12)
		pdf.SetX(20)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, label+": "+value)
		yPos += lineHeight
	}

	addTitle(cv.Name + " " + cv.Surname)
	addText("Profession", cv.Profession)
	addText("Age", strconv.Itoa(cv.Age))
	addText("Living City", cv.LivingCity)
	addText("Salary Expectation", strconv.Itoa(cv.Salary))
	addText("Email", cv.EmailCV)
	addText("Phone", cv.PhoneNumber)
	addText("Education", cv.Education)

	pdf.SetFont("LiberationSans-Bold", "", 12)
	pdf.SetX(float64(20))
	pdf.SetY(float64(yPos))
	pdf.Cell(nil, "Soft Skills:")
	yPos += 15

	soft := []string{}
	for _, sk := range cv.SoftSkills {
		soft = append(soft, strings.Fields(sk)...)
	}

	for _, skill := range soft {
		pdf.SetX(30)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "- "+skill)
		yPos += 15
	}
	yPos += 25

	pdf.SetFont("LiberationSans-Bold", "", 12)
	pdf.SetX(float64(20))
	pdf.SetY(float64(yPos))
	pdf.Cell(nil, "Hard Skills:")
	yPos += 15

	hard := []string{}
	for _, sk := range cv.HardSkills {
		hard = append(hard, strings.Fields(sk)...)
	}

	for _, skill := range hard {
		pdf.SetX(30)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "- "+skill)
		yPos += 15
	}

	if cv.Description != "" {
		yPos += 15
		pdf.SetFont("LiberationSans-Bold", "", 12)
		pdf.SetX(float64(280))
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "Brief")

		yPos += 20
		pdf.SetFont("LiberationSans-Bold", "", 12)
		pdf.SetX(float64(20))
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, cv.Description)
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=CV.pdf")

	if _, err := pdf.WriteTo(w); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		h.logg.Errorln("Error writing PDF to response: ", err)
		return
	}

	h.logg.Infoln("PDF is successfully created: CV.pdf")
}

func getUserSession(r *http.Request) (int, error) {
	token, err := r.Cookie("JWT")
	if token.Value == "" {
		return 0, errors.New("cookie is empty: session deleted")
	}
	if err != nil {
		return 0, errors.New("cookie error")
	}

	claims, err := auth.ValidateJWT(token.Value)
	if err != nil {
		return 0, errors.New("bad cookie")
	}
	return claims.UserID, nil
}

func setCookie(w http.ResponseWriter, cookieName string, cookies string, ttl time.Duration) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    cookies,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		Expires:  time.Now().UTC().Add(ttl),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

func clearCookie(w http.ResponseWriter, cookieName string) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		Secure:   true,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

func (h *Handlers) getUserCV(id int, prof string) (ent.CV, error) {
	searchCV, existed := utils.BinSearch(h.cvs, id, prof)
	if !existed {
		redisCV, err := h.srv.GetDataCV(prof)
		if err != nil {
			return ent.CV{}, err
		}
		searchCV = *redisCV
	}
	return searchCV, nil
}

func (h *Handlers) handleProfessions(Profs []string, id int) []ent.CV {
	for _, pr := range Profs {
		if cashCV, ok := h.cash[id]; ok && cashCV.Profession == pr {
			h.cvs = append(h.cvs, *cashCV)
			h.logg.Infoln("CV from cash: ", cashCV.Profession)
		} else {
			cv, err := h.srv.GetDataCV(pr)
			if err != nil {
				h.logg.Errorln("Error: ", err, " fetching CV: ", pr)
				continue
			}
			h.cash[id] = cv
			h.cvs = append(h.cvs, *cv)
			h.logg.Infoln("CV from redis: ", cv.Profession)
		}
	}
	return h.cvs
}

func (h *Handlers) checkValidRequest(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self';")

	if err := r.ParseForm(); err != nil {
		maxBytes := &http.MaxBytesError{}
		if err == maxBytes {
			http.Error(w, "Request body too large (max 10 MB)", http.StatusTooManyRequests)
			h.logg.Errorln("Request body too large (max 10 MB)")
			return err
		}
		http.Error(w, "Wrong input of data", http.StatusBadRequest)
		h.logg.Errorln(err)
		return err
	}
	return nil
}

func getUserDevice(agent string) string {
	device := func(agent string) string {
		if strings.Contains(agent, "mobile") {
			return "Mobile"
		} else if strings.Contains(agent, "tablet") {
			return "Tablet"
		} else {
			return "Desktop"
		}
	}(agent)
	return device
}
