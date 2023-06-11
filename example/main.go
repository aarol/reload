package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/aarol/reload"
)

var isDevelopment = flag.Bool("dev", false, "Enable hot-reload")

func parseTemplates() *template.Template {
	return template.Must(template.ParseGlob("ui/*.html"))
}

func main() {
	flag.Parse()

	templateCache := parseTemplates()

	reload.OnReload = func() {
		templateCache = parseTemplates()
	}

	// serve any static files like you normally would
	http.Handle("/static/", http.FileServer(http.Dir("ui/")))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// serve index.html with template data
		data := map[string]any{
			"Timestamp": time.Now().Format("Monday, 02-Jan-06 15:04:05 MST"),
		}
		err := templateCache.ExecuteTemplate(w, "index.html", data)
		if err != nil {
			fmt.Println(err)
		}
	})

	// this can be any http.Handler like chi.Router or gin.Engine
	var handler http.Handler = http.DefaultServeMux

	if *isDevelopment {
		reload.Directories = []string{"ui/"}
		handler = reload.Handle(handler)
	} else {
		fmt.Println("Running in production mode")
	}

	addr := "localhost:3001"

	fmt.Println("Server running at", addr)

	fmt.Println(http.ListenAndServe(addr, handler))
}
