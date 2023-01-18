package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aarol/reload"
)

func main() {
	templateCache := newTemplateCache()

	reloader := &reload.Reloader{
		Paths: []string{"ui/"},
		OnReload: func() {
			templateCache = newTemplateCache()
		},
		EndpointPath: "/reload",
		Disabled:     os.Getenv("MODE") == "PRODUCTION",
	}
	go reloader.Run()

	// serve any static files like you normally would
	http.Handle("/static/", http.FileServer(http.Dir("ui/")))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// an anonymous struct containing the data that
		// should be passed to the template
		data := struct {
			LiveReload template.HTML
			Timestamp  string
		}{
			LiveReload: reloader.InjectedScript(),
			Timestamp:  time.Now().Format(time.RFC850),
		}
		err := templateCache.ExecuteTemplate(w, "index.html", data)
		if err != nil {
			fmt.Println(err)
		}
	})

	http.HandleFunc(reloader.EndpointPath, reloader.ServeWS)

	log.Fatal(http.ListenAndServe("localhost:3001", nil))
}

func newTemplateCache() *template.Template {
	return template.Must(template.ParseGlob("ui/*.html"))
}
