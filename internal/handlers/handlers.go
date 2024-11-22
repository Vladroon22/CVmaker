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
	cash map[any]*service.CV
}

func NewHandler(l *golog.Logger, r *database.Repo, s *service.Service, rd *database.Redis) *Handlers {
	return &Handlers{
		logg: l,
		repo: r,
		srv:  s,
		red:  rd,
		data: make([]PageUsersCV, 0),
		cvs:  make([]service.CV, 0),
		cash: make(map[any]*service.CV),
	}
}

var tmpl = template.Must(template.ParseFiles(
	filepath.Join("web", "index.html"),
	filepath.Join("web", "cv.html"),
	filepath.Join("web", "cv-edit.html"),
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
	h.cash[cv.Profession] = cv
	h.data = append(h.data, PageUsersCV{Profession: cv.Profession})
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
			h.logg.Infoln("No CVs")
			break
		}
		if len(Profs) == len(h.data) {
			h.logg.Infoln("No new CVs")
			break
		}
		if cashCV, ok := h.cash[pr]; ok {
			h.data = append(h.data, PageUsersCV{Profession: pr})
			h.cvs = append(h.cvs, *cashCV)
			break
		} else if !ok {
			cv, err := h.repo.GetDataCV(pr)
			if err != nil {
				h.logg.Errorln("Error: ", err, " fetching CV: ", pr)
				continue
			}
			h.data = append(h.data, PageUsersCV{Profession: pr})
			h.cvs = append(h.cvs, *cv)
			break
		}
	}
	tmpl, err := template.ParseFiles("./web/cv-list.html")
	tmpl.Execute(w, h.data)
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

	if cashCV, ok := h.cash[prof]; ok {
		for i, cv := range h.cvs {
			if cv.Profession == cashCV.Profession {
				if i == 0 && len(h.data) > 0 {
					h.data = h.data[i:]
					h.cvs = h.cvs[i:]
					delete(h.cash, cashCV.Profession)
				} else {
					h.data = append(h.data[:i], h.data[i+1:]...)
					h.cvs = append(h.cvs[:i], h.cvs[i+1:]...)
					delete(h.cash, cashCV.Profession)
					break
				}
			}
		}
	} else if !ok {
		for i, cv := range h.cvs {
			if cv.Profession == prof {
				if i == 0 && len(h.data) > 0 {
					h.data = h.data[i:]
					h.cvs = h.cvs[i:]
				} else {
					h.data = append(h.data[:i], h.data[i+1:]...)
					h.cvs = append(h.cvs[:i], h.cvs[i+1:]...)
					h.red.Make("lset", "jobs", i, prof)
					h.red.Make("lrem", "jobs", i, prof)
					break
				}
			}
		}
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
		return
	}

	if err := pdf.SetFont("LiberationSans-Bold", "", 12); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		h.logg.Fatalln(err)
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
