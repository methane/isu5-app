package main

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/garyburd/redigo/redisx"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

const UnixPath = "/tmp/app.sock"

var (
	db      *sql.DB
	redisDB *redisx.ConnMux
)

func initredis() {
	for {
		conn, err := redis.Dial("tcp", ":6379")
		if err != nil {
			log.Print(err)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		redisDB = redisx.NewConnMux(conn)
		return
	}
}

type User struct {
	ID    int
	Email string
	Grade string
}

type Arg map[string]*Service

type Service struct {
	Token  string            `json:"token"`
	Keys   []string          `json:"keys"`
	Params map[string]string `json:"params"`
}

type Data struct {
	Service string                 `json:"service"`
	Data    map[string]interface{} `json:"data"`
}

var saltChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func getSession(w http.ResponseWriter, r *http.Request) int {

	for _, c := range r.Cookies() {
		if c.Name == "user_id" {
			i, err := strconv.Atoi(c.Value)
			if err != nil {
				return 0
			}
			return i
		}
	}
	return 0
}

func setSession(w http.ResponseWriter, userID int) {
	cookie := http.Cookie{
		Name:  "user_id",
		Value: fmt.Sprintf("%d", userID),
	}
	http.SetCookie(w, &cookie)
}

func getTemplatePath(file string) string {
	return path.Join("templates", file)
}

func render(w http.ResponseWriter, r *http.Request, status int, file string, data interface{}) {
	tpl := template.Must(template.New(file).ParseFiles(getTemplatePath(file)))
	w.WriteHeader(status)
	checkErr(tpl.Execute(w, data))
}

func authenticate(w http.ResponseWriter, r *http.Request, email, passwd string) *User {
	query := `SELECT id, email, grade FROM users WHERE email=$1 AND passhash=digest(salt || $2, 'sha512')`
	row := db.QueryRow(query, email, passwd)
	user := User{}
	err := row.Scan(&user.ID, &user.Email, &user.Grade)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		checkErr(err)
	}
	setSession(w, user.ID)
	context.Set(r, "user", user)
	return &user
}

func getCurrentUser(w http.ResponseWriter, r *http.Request) *User {
	u := context.Get(r, "user")
	if u != nil {
		user := u.(User)
		return &user
	}
	userID := getSession(w, r)
	if userID == 0 {
		return nil
	}
	row := db.QueryRow(`SELECT id,email,grade FROM users WHERE id=$1`, userID)
	user := User{}
	err := row.Scan(&user.ID, &user.Email, &user.Grade)
	if err == sql.ErrNoRows {
		clearSession(w, r)
		return nil
	}
	checkErr(err)
	context.Set(r, "user", user)
	return &user
}

func generateSalt() string {
	salt := make([]rune, 32)
	for i := range salt {
		salt[i] = saltChars[rand.Intn(len(saltChars))]
	}
	return string(salt)
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	setSession(w, 0)
}

func GetSignUp(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r)
	render(w, r, http.StatusOK, "signup.html", nil)
}

func PostSignUp(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	passwd := r.FormValue("password")
	grade := r.FormValue("grade")
	salt := generateSalt()
	insertUserQuery := `INSERT INTO users (email,salt,passhash,grade) VALUES ($1,$2,digest($3 || $4, 'sha512'),$5) RETURNING id`
	insertSubscriptionQuery := `INSERT INTO subscriptions (user_id,arg) VALUES ($1,$2)`
	tx, err := db.Begin()
	checkErr(err)
	row := tx.QueryRow(insertUserQuery, email, salt, salt, passwd, grade)

	var userId int
	err = row.Scan(&userId)
	if err != nil {
		tx.Rollback()
		checkErr(err)
	}
	_, err = tx.Exec(insertSubscriptionQuery, userId, "{}")
	if err != nil {
		tx.Rollback()
		checkErr(err)
	}
	checkErr(tx.Commit())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func PostCancel(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/signup", http.StatusSeeOther)
}

func GetLogin(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r)
	render(w, r, http.StatusOK, "login.html", nil)
}

func PostLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	passwd := r.FormValue("password")
	authenticate(w, r, email, passwd)
	if getCurrentUser(w, r) == nil {
		http.Error(w, "Failed to login.", http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func GetLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func GetIndex(w http.ResponseWriter, r *http.Request) {
	if getCurrentUser(w, r) == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	render(w, r, http.StatusOK, "main.html", struct{ User User }{*getCurrentUser(w, r)})
}

func GetUserJs(w http.ResponseWriter, r *http.Request) {
	if getCurrentUser(w, r) == nil {
		http.Error(w, "Failed to login.", http.StatusForbidden)
		return
	}
	render(w, r, http.StatusOK, "user.js", struct{ Grade string }{getCurrentUser(w, r).Grade})
}

func GetModify(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(w, r)
	if user == nil {
		http.Error(w, "Failed to login.", http.StatusForbidden)
		return
	}
	row := db.QueryRow(`SELECT arg FROM subscriptions WHERE user_id=$1`, user.ID)
	var arg string
	err := row.Scan(&arg)
	if err == sql.ErrNoRows {
		arg = "{}"
	}
	render(w, r, http.StatusOK, "modify.html", struct {
		User User
		Arg  string
	}{*user, arg})
}

func PostModify(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(w, r)
	if user == nil {
		http.Error(w, "Failed to login.", http.StatusForbidden)
		return
	}

	service := r.FormValue("service")
	token := r.FormValue("token")
	keysStr := r.FormValue("keys")
	keys := []string{}
	if keysStr != "" {
		keys = regexp.MustCompile("\\s+").Split(keysStr, -1)
	}
	paramName := r.FormValue("param_name")
	paramValue := r.FormValue("param_value")

	selectQuery := `SELECT arg FROM subscriptions WHERE user_id=$1 FOR UPDATE`
	updateQuery := `UPDATE subscriptions SET arg=$1 WHERE user_id=$2`

	tx, err := db.Begin()
	checkErr(err)
	row := tx.QueryRow(selectQuery, user.ID)
	var jsonStr string
	err = row.Scan(&jsonStr)
	if err == sql.ErrNoRows {
		jsonStr = "{}"
	} else if err != nil {
		tx.Rollback()
		checkErr(err)
	}
	var arg Arg
	err = json.Unmarshal([]byte(jsonStr), &arg)
	if err != nil {
		tx.Rollback()
		checkErr(err)
	}

	if _, ok := arg[service]; !ok {
		arg[service] = &Service{}
	}
	if token != "" {
		arg[service].Token = token
	}
	if len(keys) > 0 {
		arg[service].Keys = keys
	}
	if arg[service].Params == nil {
		arg[service].Params = make(map[string]string)
	}
	if paramName != "" && paramValue != "" {
		arg[service].Params[paramName] = paramValue
	}

	b, err := json.Marshal(arg)
	if err != nil {
		tx.Rollback()
		checkErr(err)
	}
	_, err = tx.Exec(updateQuery, string(b), user.ID)
	checkErr(err)

	tx.Commit()

	http.Redirect(w, r, "/modify", http.StatusSeeOther)
}

func fetchKen(name string) string {
	rc := redisDB.Get()
	{
		res, err := rc.Do("HGET", "ken", name)
		rc.Close()
		if err != nil {
			log.Println(err)
			res = nil
		}
		if res != nil {
			switch res := res.(type) {
			case string:
				return res
			case []byte:
				return string(res)
			}
		}
	}
	res, err := http.DefaultClient.Get("http://api.five-final.isucon.net:8080/" + name)
	checkErr(err)
	raw, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	sraw := string(raw)
	rc.Do("HSET", "ken", name, sraw)
	rc.Close()
	return sraw
}

func fetchName(nametype, name string) string {
	rc := redisDB.Get()
	{
		res, err := rc.Do("HGET", nametype, name)
		rc.Close()
		if err != nil {
			log.Println(err)
			res = nil
		}
		if res != nil {
			switch res := res.(type) {
			case string:
				return res
			case []byte:
				return string(res)
			}
		}
	}

	params := url.Values{}
	params.Add("q", name)

	req, err := http.NewRequest("GET", "http://api.five-final.isucon.net:8081/"+nametype, nil)
	checkErr(err)
	req.URL.RawQuery = params.Encode()

	res, err := http.DefaultClient.Do(req)
	checkErr(err)
	raw, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	sraw := string(raw)
	rc.Do("HSET", nametype, name, sraw)
	rc.Close()
	return sraw
}

func fetchApi(method, uri string, headers, params map[string]string) string {
	values := url.Values{}
	for k, v := range params {
		values.Add(k, v)
	}

	var req *http.Request
	var err error
	switch method {
	case "GET":
		req, err = http.NewRequest(method, uri, nil)
		checkErr(err)
		req.URL.RawQuery = values.Encode()
		break
	case "POST":
		req, err = http.NewRequest(method, uri, strings.NewReader(values.Encode()))
		checkErr(err)
		break
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	checkErr(err)

	raw, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return string(raw)
}

func GetData(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(w, r)
	if user == nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	row := db.QueryRow(`SELECT arg FROM subscriptions WHERE user_id=$1`, user.ID)
	var argJson string
	checkErr(row.Scan(&argJson))
	var arg Arg
	checkErr(json.Unmarshal([]byte(argJson), &arg))

	data := make([]string, 0, len(arg))
	for service, conf := range arg {
		if service == "ken" {
			res := fetchKen(conf.Keys[0])
			data = append(data, fmt.Sprintf(`{"service": %q, "data": %s}`, service, res))
			continue
		}
		if service == "ken2" {
			res := fetchKen(conf.Params["zipcode"])
			data = append(data, fmt.Sprintf(`{"service": %q, "data": %s}`, service, res))
			continue
		}
		if service == "surname" || service == "givenname" {
			res := fetchName(service, conf.Params["q"])
			data = append(data, fmt.Sprintf(`{"service": %q, "data": %s}`, service, res))
			continue
		}

		ep := Services[service]
		headers := make(map[string]string)
		params := conf.Params
		if params == nil {
			params = make(map[string]string)
		}

		if ep.TokenType != "" && ep.TokenKey != "" {
			switch ep.TokenType {
			case "header":
				headers[ep.TokenKey] = conf.Token
				break
			case "param":
				params[ep.TokenKey] = conf.Token
				break
			}
		}

		ks := make([]interface{}, len(conf.Keys))
		for i, s := range conf.Keys {
			ks[i] = s
		}
		uri := fmt.Sprintf(ep.Uri, ks...)
		res := fetchApi(ep.Meth, uri, headers, params)
		//data = append(data, Data{service, res})
		data = append(data, fmt.Sprintf(`{"service": %q, "data": %s}`, service, res))
	}

	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	first := true
	for _, s := range data {
		if !first {
			buf.WriteByte(',')
		}
		buf.WriteString(s)
	}
	buf.WriteByte(']')

	w.Header().Set("Content-Type", "application/json")
	//body, err := json.Marshal(data)
	//checkErr(err)
	w.Write(buf.Bytes())
}

func GetInitialize(w http.ResponseWriter, r *http.Request) {
	url := os.Getenv("SLACK_URL")
	if url != "" {
		_, err := http.Post(url, "text/plain", bytes.NewBuffer([]byte("Start Benchmark")))
		if err != nil {
			log.Print(err)
		}
		r.Body.Close()
	}

	fname := "../sql/initialize.sql"
	file, err := filepath.Abs(fname)
	checkErr(err)
	_, err = exec.Command("psql", "-f", file, "isucon5f").Output()
	checkErr(err)
}

func main() {
	host := os.Getenv("ISUCON5_DB_HOST")
	if host == "" {
		host = "localhost"
	}
	portstr := os.Getenv("ISUCON5_DB_PORT")
	if portstr == "" {
		portstr = "5432"
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		log.Fatalf("Failed to read DB port number from an environment variable ISUCON5_DB_PORT.\nError: %s", err.Error())
	}
	user := os.Getenv("ISUCON5_DB_USER")
	if user == "" {
		user = "isucon"
	}
	password := os.Getenv("ISUCON5_DB_PASSWORD")
	dbname := os.Getenv("ISUCON5_DB_NAME")
	if dbname == "" {
		dbname = "isucon5f"
	}

	db, err = sql.Open("postgres", "host="+host+" port="+strconv.Itoa(port)+" user="+user+" dbname="+dbname+" sslmode=disable password="+password)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %s.", err.Error())
	}
	db.SetMaxIdleConns(3)
	db.SetMaxOpenConns(3)
	defer db.Close()

	initredis()

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	r := mux.NewRouter()

	s := r.Path("/signup").Subrouter()
	s.Methods("GET").HandlerFunc(GetSignUp)
	s.Methods("POST").HandlerFunc(PostSignUp)

	l := r.Path("/login").Subrouter()
	l.Methods("GET").HandlerFunc(GetLogin)
	l.Methods("POST").HandlerFunc(PostLogin)

	r.HandleFunc("/logout", GetLogout).Methods("GET")

	m := r.Path("/modify").Subrouter()
	m.Methods("GET").HandlerFunc(GetModify)
	m.Methods("POST").HandlerFunc(PostModify)

	r.HandleFunc("/data", GetData).Methods("GET")

	r.HandleFunc("/cancel", PostCancel).Methods("POST")

	r.HandleFunc("/user.js", GetUserJs).Methods("GET")

	r.HandleFunc("/initialize", GetInitialize).Methods("GET")

	r.HandleFunc("/", GetIndex)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("../static")))

	go http.ListenAndServe(":3000", nil) // for debug

	os.Remove(UnixPath)
	ul, err := net.Listen("unix", UnixPath)
	if err != nil {
		panic(err)
	}
	os.Chmod(UnixPath, 0777)
	defer ul.Close()
	log.Fatal(http.Serve(ul, r))

	log.Fatal(http.ListenAndServe(":8080", r))
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
