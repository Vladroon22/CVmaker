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

func (h *Handlers) parseCVForm(id string, r *http.Request) (*ent.CV, error) {
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
		RequestID := utils.GenRequestID()

		var (
			ID           any = "id"
			KeyRequestID any = "X-Request-ID"
		)

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

		w.Header().Set(KeyRequestID.(string), RequestID)

		ctx := context.WithValue(r.Context(), ID, claims.UserID)
		ctx = context.WithValue(r.Context(), KeyRequestID, RequestID)

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

	pdf.SetFillColor(250, 250, 252)
	pdf.Rectangle(0, 0, 595.28, 841.89, "F", 0.0, 0)

	pdf.SetFillColor(230, 140, 75)
	pdf.Rectangle(0, 0, 595.28, 8, "F", 0.0, 0)

	pdf.SetFillColor(47, 69, 89)
	pdf.Rectangle(0, 833.89, 595.28, 8, "F", 0.0, 0)

	family := os.Getenv("family")
	if err := pdf.AddTTFFont(family, os.Getenv("ttfpath")); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	boldFamily := os.Getenv("family")
	boldPath := os.Getenv("ttfpath")
	hasBold := false
	if boldFamily != "" && boldPath != "" {
		if err := pdf.AddTTFFont(boldFamily, boldPath); err == nil {
			hasBold = true
		}
	}

	yPos := 50.0
	lineHeight := 22.0
	leftMargin := 40.0

	addSectionTitle := func(text string) {
		pdf.SetFillColor(230, 140, 75)
		pdf.Rectangle(leftMargin-10, yPos-2, 5, 18, "F", 0.0, 0)

		if hasBold {
			pdf.SetFont(boldFamily, "", 14)
		} else {
			pdf.SetFont(family, "", 14)
		}
		pdf.SetTextColor(44, 62, 80)
		pdf.SetX(leftMargin)
		pdf.SetY(yPos)
		pdf.Cell(nil, text)
		yPos += lineHeight + 10
		pdf.SetFont(family, "", 11)
		pdf.SetTextColor(60, 70, 85)
	}

	addInfoRow := func(label, value string) {
		pdf.SetFillColor(248, 249, 250)
		pdf.Rectangle(leftMargin-5, yPos-3, 500, 20, "F", 0.0, 0)

		if hasBold {
			pdf.SetFont(boldFamily, "", 11)
		} else {
			pdf.SetFont(family, "", 11)
		}
		pdf.SetTextColor(230, 140, 75)
		pdf.SetX(leftMargin)
		pdf.SetY(yPos)
		pdf.Cell(nil, label+":")

		pdf.SetFont(family, "", 11)
		pdf.SetTextColor(44, 62, 80)
		pdf.SetX(leftMargin + 120)
		pdf.SetY(yPos)
		pdf.Cell(nil, value)
		yPos += lineHeight + 5
	}

	if hasBold {
		pdf.SetFont(boldFamily, "", 24)
	} else {
		pdf.SetFont(family, "", 24)
	}
	pdf.SetTextColor(44, 62, 80)

	nameText := cv.Name + " " + cv.Surname
	nameWidth, _ := pdf.MeasureTextWidth(nameText)
	pdf.SetX((595.28 - nameWidth) / 2)
	pdf.SetY(yPos)
	pdf.Cell(nil, nameText)

	pdf.SetFillColor(230, 140, 75)
	pdf.Rectangle((595.28-100)/2, yPos+25, 100, 3, "F", 0.0, 0)

	yPos += 45

	if hasBold {
		pdf.SetFont(boldFamily, "", 16)
	} else {
		pdf.SetFont(family, "", 16)
	}
	pdf.SetTextColor(100, 120, 140)
	profText := cv.Profession
	profWidth, _ := pdf.MeasureTextWidth(profText)
	pdf.SetX((595.28 - profWidth) / 2)
	pdf.SetY(yPos)
	pdf.Cell(nil, profText)

	yPos += 35

	addSectionTitle("📋 Personal Information")

	col1Y := yPos
	addInfoRow("Age", strconv.Itoa(cv.Age))
	addInfoRow("Living City", cv.LivingCity)
	addInfoRow("Email", cv.EmailCV)

	yPos = col1Y
	leftMargin = 320.0
	addInfoRow("Phone", cv.PhoneNumber)
	addInfoRow("Education", cv.Education)
	addInfoRow("Salary Expectation", strconv.Itoa(cv.Salary)+" "+cv.Currency)

	leftMargin = 40.0
	yPos += 20

	addSectionTitle("🤝 Soft Skills")

	soft := []string{}
	for _, sk := range cv.SoftSkills {
		soft = append(soft, strings.Fields(sk)...)
	}

	skillX := leftMargin
	skillY := yPos
	for i, skill := range soft {
		if i > 0 && i%3 == 0 {
			skillY += 25
			skillX = leftMargin
		}

		pdf.SetFillColor(240, 248, 255)
		skillWidth := float64(len(skill)*7 + 30)
		pdf.Rectangle(skillX, skillY, skillWidth, 20, "F", 0.0, 0)

		pdf.SetStrokeColor(100, 180, 220)
		pdf.SetLineWidth(1)
		pdf.Rectangle(skillX, skillY, skillWidth, 20, "D", 0.0, 0)

		pdf.SetFont(family, "", 10)
		pdf.SetTextColor(44, 62, 80)
		pdf.SetX(skillX + 15)
		pdf.SetY(skillY + 4)
		pdf.Cell(nil, skill)

		skillX += skillWidth + 10
	}

	yPos = skillY + 40

	addSectionTitle("🛠️ Hard Skills")

	hard := []string{}
	for _, sk := range cv.HardSkills {
		hard = append(hard, strings.Fields(sk)...)
	}

	skillX = leftMargin
	skillY = yPos
	for i, skill := range hard {
		if i > 0 && i%3 == 0 {
			skillY += 25
			skillX = leftMargin
		}

		pdf.SetFillColor(255, 248, 240)
		skillWidth := float64(len(skill)*7 + 30)
		pdf.Rectangle(skillX, skillY, skillWidth, 20, "F", 0.0, 0)

		pdf.SetStrokeColor(230, 140, 75)
		pdf.SetLineWidth(1)
		pdf.Rectangle(skillX, skillY, skillWidth, 20, "D", 0.0, 0)

		// Текст тега
		pdf.SetFont(family, "", 10)
		pdf.SetTextColor(44, 62, 80)
		pdf.SetX(skillX + 15)
		pdf.SetY(skillY + 4)
		pdf.Cell(nil, skill)

		skillX += skillWidth + 10
	}

	yPos = skillY + 50

	if cv.Description != "" {
		addSectionTitle("📝 About Me")

		pdf.SetFillColor(248, 249, 250)
		pdf.Rectangle(leftMargin-5, yPos-5, 500, 80, "F", 0.0, 0)

		pdf.SetStrokeColor(200, 210, 220)
		pdf.SetLineWidth(0.5)
		pdf.Rectangle(leftMargin-5, yPos-5, 500, 80, "D", 0.0, 0)

		pdf.SetFont(family, "", 11)
		pdf.SetTextColor(60, 70, 85)
		pdf.SetX(leftMargin + 10)
		pdf.SetY(yPos + 5)

		words := strings.Split(cv.Description, " ")
		lineText := ""
		maxWidth := 470.0

		for _, word := range words {
			testLine := lineText
			if testLine != "" {
				testLine += " "
			}
			testLine += word

			width, _ := pdf.MeasureTextWidth(testLine)
			if width > maxWidth {
				pdf.SetX(leftMargin + 10)
				pdf.Cell(nil, lineText)
				yPos += 18
				pdf.SetY(yPos)
				lineText = word
			} else {
				lineText = testLine
			}
		}

		if lineText != "" {
			pdf.SetX(leftMargin + 10)
			pdf.Cell(nil, lineText)
		}
	}

	pdf.SetFont(family, "", 9)
	pdf.SetTextColor(150, 160, 170)
	footerText := "Generated by CV Maker • " + time.Now().Format("January 2, 2006")
	footerWidth, _ := pdf.MeasureTextWidth(footerText)
	pdf.SetX((595.28 - footerWidth) / 2)
	pdf.SetY(810)
	pdf.Cell(nil, footerText)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=CV.pdf")

	if _, err := pdf.WriteTo(w); err != nil {
		http.Error(w, "Error of creating pdf-file", http.StatusInternalServerError)
		log.Println("Error writing PDF to response: ", err)
		return
	}

	log.Println("PDF is successfully created: CV.pdf")
}

func getUserSession(r *http.Request) (string, error) {
	token, err := r.Cookie("JWT")
	if token.Value == "" {
		return "", errors.New("cookie is empty: session deleted")
	}
	if err != nil {
		return "", errors.New("cookie error")
	}

	claims, err := auth.ValidateJWT(token.Value)
	if err != nil {
		return "", errors.New("bad cookie")
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

func (h *Handlers) getUserCV(id string, prof string) (*ent.CV, error) {
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

func (h *Handlers) handleProfessions(Profs []string, id string) []ent.CV {
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
