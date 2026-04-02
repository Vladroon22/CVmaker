package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Vladroon22/CVmaker/internal/auth"
	"github.com/Vladroon22/CVmaker/internal/cache"
	ent "github.com/Vladroon22/CVmaker/internal/entity"
	"github.com/Vladroon22/CVmaker/internal/service"
	"github.com/Vladroon22/CVmaker/internal/utils"
	"github.com/signintech/gopdf"
)

type PageData struct {
	Error error
}

type Handlers struct {
	srv  service.Servicer
	cash *cache.Cache
}

func NewHandler(s service.Servicer) *Handlers {
	return &Handlers{
		srv:  s,
		cash: cache.InitCache(),
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
		log.Println(err)
		return
	}

	user := ent.UserInput{}
	user.Name = r.FormValue("username")
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := utils.Valid(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println(err)
		return
	}

	if err := h.srv.CreateUser(r.Context(), &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	if err := h.checkValidRequest(w, r); err != nil {
		log.Println(err)
		return
	}

	user := ent.UserInput{}
	device := getUserDevice(r.Header.Get("User-Agent"))
	user.Password = r.FormValue("password")
	user.Email = r.FormValue("email")

	if err := utils.Valid(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println(err)
		return
	}

	id, err := h.srv.Login(r.Context(), user.Password, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		log.Println(err)
		return
	}

	if err := h.srv.SaveSession(r.Context(), id, device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	token, err := auth.GenerateJWT(id)
	if err != nil {
		http.Error(w, "Error of creating token-session", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	setCookie(w, "JWT", token, utils.TTLofJWT)
	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
}

func (h *Handlers) parseCVForm(id int, r *http.Request) (*ent.CV, error) {
	cv := &ent.CV{}

	age := r.FormValue("age")
	if !utils.ValidateDataAge(age) {
		log.Println("Not valid data of birth")
		return nil, errors.New("not valid data of birth")
	}

	PhoneNumber := r.FormValue("phone")
	if ok := utils.ValidatePhone(PhoneNumber); !ok {
		log.Println("Wrong phone number input in CV")
		return nil, errors.New("wrong phone number input in CV")
	}

	email := r.FormValue("emailcv")
	if ok := utils.ValidateEmail(email); !ok {
		log.Println("Wrong email input in CV")
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
		log.Println(err)
		return nil, errors.New("salary set in wrong format")
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
	id, err := getUserSession(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Println(err)
		return
	}

	if err := h.checkValidRequest(w, r); err != nil {
		log.Println(err)
		return
	}

	parsedCV, err := h.parseCVForm(id, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.srv.AddNewCV(parsedCV); err != nil {
		http.Error(w, "CV's data sent incorrectly", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	h.cash.Set(parsedCV.ID, parsedCV)

	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
}

func (h *Handlers) ListCV(w http.ResponseWriter, r *http.Request) {
	id, err := getUserSession(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Println(err)
		return
	}

	Profs, err := h.srv.GetProfessions(id)
	if err != nil {
		http.Error(w, "Profession's data got incorrectly", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	cachedCVs := h.cash.FromToSliceByID(id)
	if len(Profs) == 0 {
		log.Println("No CVs in redis")
		renderTemplate(w, "./web/cv-list.html", cachedCVs)
		return
	}

	if len(Profs) == h.cash.GetLen(id) {
		log.Println("No new CVs")
		renderTemplate(w, "./web/cv-list.html", cachedCVs)
		return
	}

	log.Println("Professions: ", Profs)

	cvs := h.handleProfessions(Profs, id)

	log.Println("CVs: ", len(cvs))
	renderTemplate(w, "./web/cv-list.html", cvs)
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
	id, err := getUserSession(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Println(err)
		return
	}

	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		log.Println("Profession not provided")
		return
	}

	searchCV, err := h.getUserCV(id, prof)
	if err != nil {
		http.Error(w, "Error of receive data", http.StatusInternalServerError)
		log.Println("Error of receive data from redis: ", err)
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
	id, err := getUserSession(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Println(err)
		return
	}

	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		log.Println("Profession not provided")
		return
	}

	log.Println("prof: " + prof)

	if _, ok := h.cash.Get(prof, id); !ok {
		log.Printf("profession: %s not exists", prof)
	} else {
		h.cash.Delete(prof, id)
		log.Println("deleted element with from cache")
	}

	if err := h.srv.DeleteCV(id, prof); err != nil {
		http.Error(w, "error of deleting", http.StatusInternalServerError)
		log.Printf("redis error: %v", err)
		return
	}
	log.Println("deleted element from redis")

	http.Redirect(w, r, "/user/listCV", http.StatusSeeOther)
	renderTemplate(w, "./web/cv-list.html", h.cash.FromToSliceByID(id))
}

func (h *Handlers) AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ID any = "id"
		cookieJWT, err := r.Cookie("JWT")
		if err == http.ErrNoCookie {
			http.Error(w, "Session expired", http.StatusUnauthorized)
			log.Println("Session expired: ", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		claims, err := auth.ValidateJWT(cookieJWT.Value)
		if err != nil {
			http.Error(w, "Invalid JWT", http.StatusUnauthorized)
			log.Println("Invalid JWT: ", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), ID, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

/*
func (h *Handlers) EditCV(w http.ResponseWriter, r *http.Request) {
	id, err := getUserSession(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		log.Println(err)
		return
	}

	prof := r.URL.Query().Get("profession")
	if prof == "" {
		http.Error(w, "Profession not provided", http.StatusBadRequest)
		log.Println("Profession not provided")
		return
	}

	_, err = h.getUserCV(id, prof)
	if err != nil {
		http.Error(w, "Error of receive data", http.StatusInternalServerError)
		log.Println("Error of receive data from redis: ", err)
		return
	}

}
*/

func (h *Handlers) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	id, err := getUserSession(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Println(err)
		return
	}

	profession := r.URL.Query().Get("profession")
	log.Println("Converting in pdf... ", profession)
	if profession == "" {
		http.Error(w, "profession provided", http.StatusBadRequest)
		log.Println("profession not provided")
		return
	}

	cv, err := h.getUserCV(id, profession)
	if err != nil {
		http.Error(w, "Error of receive data", http.StatusInternalServerError)
		log.Println("Error of receive data from redis: ", err)
		return
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	family := os.Getenv("family")
	if err := pdf.AddTTFFont(family, os.Getenv("ttfpath")); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	if err := pdf.SetFont(family, "", 12); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	yPos := 20
	lineHeight := 27

	addTitle := func(text string) {
		pdf.SetFont(family, "", 16)
		pdf.SetX(230)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, text)
		yPos += lineHeight + 15
	}

	addText := func(label, value string) {
		pdf.SetFont(family, "", 12)
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
	yPos += 15

	soft := []string{}
	for _, sk := range cv.SoftSkills {
		soft = append(soft, strings.Fields(sk)...)
	}

	pdf.SetFont(family, "", 12)
	pdf.SetX(float64(20))
	pdf.SetY(float64(yPos))
	pdf.Cell(nil, "Soft Skills")
	yPos += 15

	for _, skill := range soft {
		pdf.SetX(30)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "* "+skill)
		yPos += 15
	}
	yPos += 25

	pdf.SetFont(family, "", 12)
	pdf.SetX(float64(20))
	pdf.SetY(float64(yPos))
	pdf.Cell(nil, "Hard Skills")
	yPos += 15

	hard := []string{}
	for _, sk := range cv.HardSkills {
		hard = append(hard, strings.Fields(sk)...)
	}

	for _, skill := range hard {
		pdf.SetX(30)
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "* "+skill)
		yPos += 15
	}

	if cv.Description != "" {
		yPos += 15
		pdf.SetFont(family, "", 12)
		pdf.SetX(float64(30))
		pdf.SetY(float64(yPos))
		pdf.Cell(nil, "Brief")

		yPos += 20
		pdf.SetFont(family, "", 12)
		pdf.SetX(float64(20))
		pdf.SetY(float64(yPos))

		words := strings.Split(cv.Description, " ")
		totalWords := len(words)

		y := pdf.GetY()

		for i, word := range words {
			IsBreak := false

			if strings.HasSuffix(word, ".") {
				IsBreak = true
			}

			if i > 0 && i%30 == 0 {
				IsBreak = true
			}

			if IsBreak {
				y++
				pdf.SetY(float64(y))
			}

			pdf.Cell(nil, word)

			if i < totalWords-1 && !IsBreak {
				pdf.Cell(nil, " ")
			}
		}
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=CV.pdf")

	if _, err := pdf.WriteTo(w); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		log.Println("Error writing PDF to response: ", err)
		return
	}

	log.Println("PDF is successfully created: CV.pdf")
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

func (h *Handlers) getUserCV(id int, prof string) (*ent.CV, error) {
	searchCV, existed := h.cash.Get(prof, id)
	if !existed {
		redisCV, err := h.srv.GetDataCV(id, prof)
		if err != nil {
			return nil, err
		}
		searchCV = redisCV
	}
	return searchCV, nil
}

func (h *Handlers) handleProfessions(Profs []string, id int) []ent.CV {
	CVs := make([]ent.CV, 0, len(Profs))

	for _, pr := range Profs {
		if cv, ok := h.cash.Get(pr, id); ok {
			CVs = append(CVs, *cv)
			log.Println("CV from cache: ", cv.Profession)
			continue
		}

		cv, err := h.srv.GetDataCV(id, pr)
		if err != nil {
			log.Println("Error: ", err, " fetching CV from Redis: ", pr)
			continue
		}

		if cv == nil {
			log.Println("Received nil CV from Redis: ", pr)
			continue
		}

		h.cash.Set(id, cv)
		CVs = append(CVs, *cv)

		log.Println("CV from redis: ", cv.Profession)
	}

	return CVs
}

func (h *Handlers) checkValidRequest(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self';")

	if err := r.ParseForm(); err != nil {
		maxBytes := &http.MaxBytesError{}
		if err == maxBytes {
			http.Error(w, "Request body too large (max 10 MB)", http.StatusTooManyRequests)
			log.Println("Request body too large (max 10 MB)")
			return err
		}
		http.Error(w, "Wrong input of data", http.StatusBadRequest)
		log.Println(err)
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
