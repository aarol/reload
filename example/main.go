package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/aarol/reload"
)

func main() {
	isDevelopment := flag.Bool("dev", true, "Development mode")
	flag.Parse()

	templateCache := parseTemplates()

	// serve any static files like normal
	http.Handle("/static/", http.FileServer(http.Dir("ui/")))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// serve a template file with dynamic data
		data := map[string]any{
			"Timestamp": time.Now().Format("Monday, 02-Jan-06 15:04:05 MST"),
		}
		err := templateCache.ExecuteTemplate(w, "index.html", data)
		if err != nil {
			fmt.Println(err)
		}
	})

	// handler can be anything that implements http.Handler,
	// like chi.Router, echo.Echo or gin.Engine
	var handler http.Handler = http.DefaultServeMux

	if *isDevelopment {
		// Call `New()` with a list of directories to recursively watch
		reload := reload.New("ui/")

		reload.OnReload = func() {
			templateCache = parseTemplates()
		}

		reload.DebugLog = log.New(os.Stderr, "reload: ", log.Lmsgprefix|log.Ltime)

		handler = reload.Handle(handler)
	} else {
		fmt.Println("Running in production mode")
	}

	addr := "localhost:3001"

	fmt.Printf("Server running at http://%s\n", addr)

	http.ListenAndServe(addr, handler)
}

func parseTemplates() *template.Template {
	return template.Must(template.ParseGlob("ui/*.html"))
}
