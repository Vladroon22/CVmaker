package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Vladroon22/CVmaker/internal/database"
	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/Vladroon22/CVmaker/internal/ut"
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
	//	data []PageUsersCV
	cvs  []service.CV
	cash map[string]*service.CV
}

func NewHandler(l *golog.Logger, r *database.Repo, s *service.Service, rd *database.Redis) *Handlers {
	return &Handlers{
		logg: l,
		repo: r,
		srv:  s,
		red:  rd,
		//		data: make([]PageUsersCV, 0),
		cvs:  make([]service.CV, 0),
		cash: make(map[string]*service.CV),
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

	if err := ut.Valid(&user); err != nil {
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

	age := r.FormValue("age")
	if !ut.ValidateDataAge(age) {
		http.Error(w, "Not valid data of birth", http.StatusBadRequest)
		h.logg.Errorln("Not valid data of birth")
		return
	}

	parts := strings.Split(age, ".")
	day, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	year, _ := strconv.Atoi(parts[2])
	tm := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	salary := r.FormValue("salary")
	cv.Profession = r.FormValue("profession")
	cv.Age = ut.CountUserAge(tm)
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
	h.cash[cv.Profession] = cv
	http.Redirect(w, r, "/user/listCV", http.StatusMovedPermanently)
}

func (h *Handlers) ListCV(w http.ResponseWriter, r *http.Request) {
	Profs, err := h.repo.GetProfessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Errorln(err)
		return
	}

	for _, pr := range Profs {
		if len(Profs) == 0 {
			h.logg.Infoln("No CVs in redis")
			break
		}
		if len(Profs) == len(h.cvs) {
			h.logg.Infoln("No new CVs")
			break
		}
		if cashCV, ok := h.cash[pr]; ok {
			h.cvs = append(h.cvs, *cashCV)
			h.logg.Infoln("CV got from cash")
			break
		} else if !ok {
			cv, err := h.repo.GetDataCV(pr)
			if err != nil {
				h.logg.Errorln("Error: ", err, " fetching CV: ", pr)
				continue
			}
			h.cvs = append(h.cvs, *cv)
			h.logg.Infoln("CV got from redis")
			break
		}
	}
	tmpl, err := template.ParseFiles("./web/cv-list.html")
	tmpl.Execute(w, h.cvs)
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
	if cashCV, ok := h.cash[prof]; ok {
		if cashCV.Profession == prof {
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
	viewHandler(w, "cv.html", searchCV)
}

func (h *Handlers) LogOut(w http.ResponseWriter, r *http.Request) {
	h.red.Make("del", "JWT")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) DeleteCV(w http.ResponseWriter, r *http.Request) {
	prof := r.URL.Query().Get("profession")
	h.logg.Infoln("prof: " + prof)

	for i, cv := range h.cvs {
		if cv.Profession == prof {
			if _, ok := h.cash[prof]; ok {
				delete(h.cash, prof)
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
		token, err := h.red.GetData("JWT")
		if token == "" {
			http.Error(w, "JWT not exists", http.StatusUnauthorized)
			h.logg.Errorln("JWT not exists")
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			h.logg.Errorln(err)
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

// not used yet
func (h *Handlers) EditCV(w http.ResponseWriter, r *http.Request) {
	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		h.logg.Errorln("Profession not provided")
		return
	}

	searchCV := &h.srv.CV
	if cashCV, ok := h.cash[prof]; ok {
		if cashCV.Profession == prof {
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

func (h *Handlers) DownLoadPDF(w http.ResponseWriter, r *http.Request) {
	profession := r.URL.Query().Get("profession")
	h.logg.Infoln(profession)
	if profession == "" {
		http.Error(w, "profession not provided", http.StatusBadRequest)
		h.logg.Errorln("profession not provided")
		return
	}

	cv := h.srv.CV
	if cashCV, ok := h.cash[profession]; ok {
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
	h.logg.Infoln(cv)

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	if err := pdf.AddTTFFont("LiberationSans-Bold", "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalln(err)
	}

	if err := pdf.SetFont("LiberationSans-Bold", "", 12); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalln(err)
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
	addText("Age", strconv.Itoa(age)) // получается ноль
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
