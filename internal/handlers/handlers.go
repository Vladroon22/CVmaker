package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/service"
	golog "github.com/Vladroon22/GoLog"
	"github.com/signintech/gopdf"
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

	if err := database.Valid(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.logg.Errorln(err)
		return
	}

	id, err := h.repo.Login(user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	token, err := h.repo.GenerateJWT(id)
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

	if err := h.repo.AddNewCV(cv); err != nil {
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

	for i, pr := range Profs {
		if len(Profs) == 0 {
			h.logg.Infoln("No CVs")
			break
		}
		if len(Profs) == len(h.data) {
			h.logg.Infoln("No new CVs")
			break
		}
		cv, err := h.repo.GetDataCV(pr)
		if cv == nil {
			h.logg.Infoln("'" + pr + "'" + " doesn't exist")
			h.red.Make("lset", "jobs", i, pr)
			h.red.Make("lrem", "jobs", i, pr)
			break
		}
		if err != nil {
			h.logg.Errorln("Error: ", err, " fetching CV: ", pr)
			continue
		}
		h.data = append(h.data, PageUsersCV{Profession: pr})
		h.cvs = append(h.cvs, *cv)
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

func (h *Handlers) DeleteCV(w http.ResponseWriter, r *http.Request) {
	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		h.logg.Errorln("Profession not provided")
		return
	}
	for i, cv := range h.cvs {
		if cv.Profession == prof {
			h.red.Make("lset", "jobs", i, cv.Profession)
			h.red.Make("lrem", "jobs", i, cv.Profession)
			break
		}
	}
	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)

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

func (h *Handlers) EditCV(w http.ResponseWriter, r *http.Request) {
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
	tmpl, err := template.ParseFiles("./web/cv-edit.html")
	tmpl.Execute(w, searchCV)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}
}

func (h *Handlers) DownLoadPDF(w http.ResponseWriter, r *http.Request) {
	profession := r.URL.Query().Get("profession")
	h.logg.Infoln(profession)
	if profession == "" {
		http.Error(w, "profession not provided", http.StatusBadRequest)
		h.logg.Errorln("profession not provided")
		return
	}

	URL := "http://" + r.Host + "/user/profile?profession=" + profession
	h.logg.Infoln(URL)

	resp, err := http.Get(URL)
	if err != nil {
		h.logg.Errorln(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to retrieve the page", http.StatusInternalServerError)
		h.logg.Fatalf("Can't get a page -> StatusCode: %d\n", resp.StatusCode)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalf("Error parsing HTML: %v\n", err)
		return
	}

	name := doc.Find("h1").Text()
	age := doc.Find("p strong").Eq(0).Next().Text()
	livingCity := doc.Find("p strong").Eq(2).Next().Text()
	salary := doc.Find("p strong").Eq(3).Next().Text()
	email := doc.Find("p strong").Eq(4).Next().Text()
	phone := doc.Find("p strong").Eq(5).Next().Text()
	education := doc.Find("p strong").Eq(6).Next().Text()

	var skills []string
	doc.Find("ul li").Each(func(i int, s *goquery.Selection) {
		skills = append(skills, s.Text())
	})

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	if err := pdf.AddTTFFont("openSans", "/usr/share/fonts/truetype/libreoffice/opens___.ttf"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalln(err)
		return
	}

	if err := pdf.SetFont("openSans", "", 14); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalln(err)
		return
	}

	content := name + "'s CV\n\n" +
		"Age: " + age + "\n" +
		"Profession: " + profession + "\n" +
		"Living City: " + livingCity + "\n" +
		"Salary Expectation: " + salary + "\n" +
		"Email: " + email + "\n" +
		"Phone: " + phone + "\n" +
		"Education: " + education + "\n\n"

	content += "Skills:\n"
	for _, skill := range skills {
		content += "- " + skill + "\n"
	}

	pdf.SetX(20)
	pdf.SetY(20)
	pdf.Cell(nil, name+"'s CV")

	pdf.SetY(40)
	pdf.Cell(nil, "Age: "+age)

	pdf.SetY(60)
	pdf.Cell(nil, "Profession: "+profession)

	pdf.SetY(80)
	pdf.Cell(nil, "Living City: "+livingCity)

	pdf.SetY(100)
	pdf.Cell(nil, "Salary Expectation: "+salary)

	pdf.SetY(120)
	pdf.Cell(nil, "Email: "+email)

	pdf.SetY(140)
	pdf.Cell(nil, "Phone: "+phone)

	pdf.SetY(160)
	pdf.Cell(nil, "Education: "+education)

	pdf.SetY(180)
	pdf.Cell(nil, "Skills:")

	yPos := 200
	for _, skill := range skills {
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "- "+skill)
		yPos += 20
	}

	if err := pdf.Cell(nil, content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalln(err)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=CV.pdf")

	if _, err := pdf.WriteTo(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalln("Error writing PDF to response:", err)
		return
	}

	h.logg.Infoln("PDF is successfully created: CV.pdf")
}

func WriteJSON(w http.ResponseWriter, status int, a any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(a)
}
