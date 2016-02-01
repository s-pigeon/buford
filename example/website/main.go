package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"flag"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/RobotsAndPencils/buford/certificate"
	"github.com/RobotsAndPencils/buford/payload"
	"github.com/RobotsAndPencils/buford/push"
	"github.com/RobotsAndPencils/buford/pushpackage"
	"github.com/gorilla/mux"
)

var (
	website = pushpackage.Website{
		Name:                "Buford",
		PushID:              "web.com.github.RobotsAndPencils.buford",
		AllowedDomains:      []string{"https://9aea51d1.ngrok.io"},
		URLFormatString:     `https://9aea51d1.ngrok.io/%@?q=%@`,
		AuthenticationToken: "19f8d7a6e9fb8a7f6d9330dabe",
		WebServiceURL:       "https://9aea51d1.ngrok.io",
	}

	templates = template.Must(template.ParseFiles("index.html", "request.html"))
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html", website)
}

func requestPermissionHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "request.html", website)
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	p := payload.Browser{
		Alert: payload.BrowserAlert{
			Title: "Hello",
			Body:  "Hello HTTP/2",
		},
		// URLArgs must match placeholders in URLFormatString
		URLArgs: []string{"click", "hello"},
	}

	id, err := Service.Push(DeviceToken, nil, p)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("apns-id:", id)
}

func clickHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("clicked", r.URL.Query()["q"])
}

// MustOpen a file or fail.
func MustOpen(name string) *os.File {
	f, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func pushPackagesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	log.Println(vars["websitePushID"])

	w.Header().Set("Content-Type", "application/zip")

	const iconPath = "../../pushpackage/fixtures/"

	icon128x := MustOpen(filepath.Join(iconPath, "gopher.png"))
	defer icon128x.Close()
	icon128 := MustOpen(filepath.Join(iconPath, "gopher.png"))
	defer icon128.Close()
	icon32x := MustOpen(filepath.Join(iconPath, "gopher.png"))
	defer icon32x.Close()
	icon32 := MustOpen(filepath.Join(iconPath, "gopher.png"))
	defer icon32.Close()
	icon16x := MustOpen(filepath.Join(iconPath, "gopher.png"))
	defer icon16x.Close()
	icon16 := MustOpen(filepath.Join(iconPath, "gopher.png"))
	defer icon16.Close()

	iconset := pushpackage.IconSet{
		{Name: "icon_128x128@2x.png", Reader: icon128x},
		{Name: "icon_128x128.png", Reader: icon128},
		{Name: "icon_32x32@2x.png", Reader: icon32x},
		{Name: "icon_32x32.png", Reader: icon32},
		{Name: "icon_16x16@2x.png", Reader: icon16x},
		{Name: "icon_16x16.png", Reader: icon16},
	}

	if err := pushpackage.New(w, &website, iconset, Cert, PrivateKey); err != nil {
		log.Fatal(err)
	}
}

func registerDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	log.Printf("register device %s for %s", vars["deviceToken"], vars["websitePushID"])

	DeviceToken = vars["deviceToken"]
}

func forgetDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	log.Printf("forget device %s for %s", vars["deviceToken"], vars["websitePushID"])

	DeviceToken = ""
}

func logHandler(w http.ResponseWriter, r *http.Request) {
	var logs struct {
		Logs []string `json:"logs"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&logs); err == io.EOF {
		return
	} else if err != nil {
		log.Fatal(err)
	}

	for _, msg := range logs.Logs {
		log.Println(msg)
	}
}

// Cert and private key for signing push packages.
var (
	Cert       *x509.Certificate
	PrivateKey *rsa.PrivateKey

	Service     push.Service
	DeviceToken string
)

func main() {
	var filename, password string

	flag.StringVar(&filename, "c", "", "Path to p12 certificate file")
	flag.StringVar(&password, "p", "", "Password for p12 file.")
	flag.Parse()

	var err error
	Cert, PrivateKey, err = certificate.Load(filename, password)
	if err != nil {
		log.Fatal(err)
	}

	Service = push.Service{
		Client: push.NewClient(certificate.TLS(Cert, PrivateKey)),
		Host:   push.Production,
	}

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/request", requestPermissionHandler)
	r.HandleFunc("/push", pushHandler)
	r.HandleFunc("/click", clickHandler).Methods("GET")

	r.HandleFunc("/v1/pushPackages/{websitePushID}", pushPackagesHandler).Methods("POST")
	r.HandleFunc("/v1/devices/{deviceToken}/registrations/{websitePushID}", registerDeviceHandler).Methods("POST")
	r.HandleFunc("/v1/devices/{deviceToken}/registrations/{websitePushID}", forgetDeviceHandler).Methods("DELETE")
	r.HandleFunc("/v1/log", logHandler).Methods("POST")

	http.ListenAndServe(":5000", r)
}