package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/aarol/reload"
)

func main() {
	templateCache := newTemplateCache()
	reload.Paths = []string{"ui/"}

	reload.OnReload = func() {
		templateCache = newTemplateCache()
	}

	// isDebug := os.Getenv("MODE") == "development"
	isDebug := true

	if isDebug {
		go reload.Run()
	}

	// serve any static files like you normally would
	http.Handle("/static/", http.FileServer(http.Dir("ui/")))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// an anonymous struct containing the data that
		// should be passed to the template
		data := map[string]any{
			"LiveReload": reload.InjectedScript("/reload"),
			"Timestamp":  time.Now().Format(time.RFC850),
		}
		err := templateCache.ExecuteTemplate(w, "index.html", data)
		if err != nil {
			fmt.Println(err)
		}
	})

	http.HandleFunc("/reload", reload.ServeWS)

	log.Fatal(http.ListenAndServe("localhost:3001", nil))
}

func newTemplateCache() *template.Template {
	return template.Must(template.ParseGlob("ui/*.html"))
}
