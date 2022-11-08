package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const CACHE_DIR = "cache/"
const HOST = "https://secure.kotter.wmulder.nl"

var mq = make(chan Msg, 20)

type Msg struct {
	ID         string
	Recipient  string // email
	Subject    string
	Body       string
	Attachment string
	Pass       string
}

func (m Msg) URL() string {
	return HOST + "/listen/" + m.ID
}

func (m Msg) String() string {
	return fmt.Sprintf("%s \"%s\"", m.Subject, m.Body)
}

var tpl struct {
	index, listen, new, mail *template.Template
}

const TemplatePath = "templates/"

func worker(f func(Msg) error, mq chan Msg) {
	var err error
	for msg := range mq {
		// time.Sleep(time.Duration(rand.Float32()*4) * time.Second)
		err = f(msg)
		if err != nil {
			log.Println(err)
		}
	}
}

// sendMail is a helper function to send an email with the test mailaccount vaccinatieregister@riseup.net.
func sendMail(to []string, subject string, body string) {
	from := "vaccinatieregister@riseup.net"
	usr := "vaccinatieregister"
	password := "Eelco!"

	host := "mail.riseup.net"
	port := "587"
	addr := host + ":" + port

	body = strings.ReplaceAll(body, "\n", "\r\n")
	msg := []byte(
		"Subject: " + subject + "\r\n" +
			"To: " + to[0] + "\r\n" +
			"From: " + from + "\r\n" +
			"Date: " + time.Now().Format("Mon Jan 02 15:04:05 -0700 2006") + "\r\n" +
			body)

	auth := smtp.PlainAuth("", usr, password, host)

	err := smtp.SendMail(addr, auth, from, to, msg)
	if err != nil {
		panic(err)
	}
}

func saveMsg(msg Msg) (err error) {
	f, err := os.Create(fmt.Sprintf("%s%s.json", CACHE_DIR, msg.ID))
	if err != nil {
		return err
	}
	bs, _ := json.MarshalIndent(msg, "", "  ")
	io.WriteString(f, string(bs))
	f.Close()
	bf := new(bytes.Buffer)
	if err = tpl.mail.Execute(bf, msg); err != nil {
		return err
	}
	sendMail([]string{msg.Recipient}, "[VR] "+msg.Subject, bf.String())
	log.Printf("Message (%s) saved and send\n", msg.ID)
	return nil
}

func readMsg(id string) (msg Msg, err error) {
	body, err := os.ReadFile(fmt.Sprintf("%s%s.json", CACHE_DIR, id))
	if err != nil {
		return Msg{}, err
	}
	err = json.Unmarshal(body, &msg)
	if err != nil {
		return Msg{}, err
	}
	return msg, nil
}

var validPath = regexp.MustCompile(`^/(listen|attach)/([a-zA-Z0-9\-]+)$`)

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("invalid Page Title")
	}
	return m[2], nil // The title is the second subexpression.
}

func getAuth(w http.ResponseWriter, r *http.Request, msg Msg) error {
	user, pass, ok := r.BasicAuth()
	// fmt.Println("username: ", user)
	// fmt.Println("password: ", pass)
	if !ok || !checkUsernameAndPassword(user, pass, msg) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Please enter your username and password for this site (username=vr)"`)
		w.WriteHeader(401)
		w.Write([]byte("Unauthorised.\n"))
		return errors.New("unauthorised")
	}
	return nil
}

func checkUsernameAndPassword(username, password string, msg Msg) bool {
	return username == "vr" && password == msg.Pass
}

func showAttachment(w http.ResponseWriter, req *http.Request) {
	title, err := getTitle(w, req)
	if err != nil {
		return
	}
	m, err := readMsg(title)
	if err != nil {
		http.Error(w, "invalid upload file: "+err.Error(), 500)
		return
	}
	err = getAuth(w, req, m)
	if err != nil {
		return
	}
	bs, _ := base64.StdEncoding.DecodeString(m.Attachment)
	bsr := bytes.NewReader(bs)
	http.ServeContent(w, req, m.Subject, time.Now(), bsr)
}

func showMsg(w http.ResponseWriter, req *http.Request) {
	title, err := getTitle(w, req)
	if err != nil {
		return
	}
	m, err := readMsg(title)
	if err != nil {
		http.Error(w, "invalid upload file: "+err.Error(), 500)
		return
	}
	err = getAuth(w, req, m)
	if err != nil {
		return
	}
	tpl.listen.ExecuteTemplate(w, "base", m)
}

func sendMsg(w http.ResponseWriter, req *http.Request) {
	subject := req.FormValue("subject")
	recipient := req.FormValue("recipient")
	body := req.FormValue("body")
	f, _, err := req.FormFile("attachment")
	if err != nil {
		http.Error(w, "invalid upload file: "+err.Error(), 500)
		return
	}
	bs, _ := io.ReadAll(f)
	attachment := base64.StdEncoding.EncodeToString(bs)
	id := uuid.New().String()
	pass := fmt.Sprintf("%5d", rand.Intn(99999))
	mq <- Msg{ID: id, Recipient: recipient, Subject: subject, Body: body, Attachment: attachment, Pass: pass}
	http.Redirect(w, req, "/", http.StatusFound)
}

func showForm(w http.ResponseWriter, req *http.Request) {
	tpl.new.ExecuteTemplate(w, "base", nil)
}

func listMsg(w http.ResponseWriter, req *http.Request) {
	fs, _ := os.ReadDir(CACHE_DIR)
	ls := make([]string, len(fs))
	for i, f := range fs {
		ls[i] = f.Name()[:len(f.Name())-len(".json")]
	}
	tpl.index.ExecuteTemplate(w, "base", ls)
}

func init() {
	os.Mkdir(CACHE_DIR, os.ModePerm)

	base := filepath.Join(TemplatePath, "base.gohtml")
	new := filepath.Join(TemplatePath, "new.gohtml")
	index := filepath.Join(TemplatePath, "index.gohtml")
	listen := filepath.Join(TemplatePath, "listen.gohtml")
	mail := filepath.Join(TemplatePath, "mail.gohtml")
	tpl.new = template.Must(template.ParseFiles(base, new))
	tpl.index = template.Must(template.ParseFiles(base, index))
	tpl.listen = template.Must(template.ParseFiles(base, listen))
	tpl.mail = template.Must(template.ParseFiles(mail))
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler.ServeHTTP(w, r)

		duration := time.Since(start)
		log.Printf("%4s %10s %s\n", r.Method, duration, r.URL)
	})
}

func main() {
	rand.Seed(time.Now().UnixNano())
	go worker(saveMsg, mq)
	go worker(saveMsg, mq)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	http.HandleFunc("/", listMsg)
	http.HandleFunc("/say", sendMsg)
	http.HandleFunc("/attach/", showAttachment)
	http.HandleFunc("/listen/", showMsg)
	http.HandleFunc("/speak", showForm)

	host := ":9999"
	log.Println("Listening on:", host)
	log.Fatal(http.ListenAndServe(host, logRequest(http.DefaultServeMux)))
}
