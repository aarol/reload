package main

import (
	"flag"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/aarol/reload"
)

func main() {
	isDevelopment := flag.Bool("dev", true, "Enable hot-reload")
	flag.Parse()

	templateCache := parseTemplates()

	reload.OnReload = func() {
		templateCache = parseTemplates()
	}

	// serve any static files like normal
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

	fmt.Printf("Server running at http://%s\n", addr)

	http.ListenAndServe(addr, handler)
}

func parseTemplates() *template.Template {
	return template.Must(template.ParseGlob("ui/*.html"))
}
