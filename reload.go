// Package reload exposes the middleware Handle(), which can be used to trigger a reload
// in the browser whenever a file is changed.
//
// Reload doesn't require any external tools and is can be
// integrated into any project that uses the standard net/http interface.
//
// Typically, integrating this package looks like this:
//
// 1. Insert the Handle() middleware at the top of the request chain and set the directories that should be watched
//
//	var handler http.Handler = http.DefaultServeMux
//
//	if isDevelopment {
//		reload.Directories = []string{"ui/"}
//		handler = reload.Handle(handler)
//	}
//
//	http.ListenAndServe(addr, handler)
//
// 2. (Optional) Use the reload.OnReload callback to re-parse any cached templates
//
//	reload.OnReload = func() {
//		app.templateCache = parseTemplates()
//	}
//
// The package also exposes `ServeWS`, `InjectScript`, `Wait` and `WatchDirectories`,
// which can be used to embed the script in the templates directly.
//
// See the full example at https://github.com/aarol/reload/blob/main/example/main.go
package reload

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

const wsCurrentVersion = "1"

type Reloader struct {
	// OnReload will be called after a file changes, but before the browser reloads.
	OnReload func()
	// A slice of directories that will be recursively watched for changes.
	// This should always be set before using the handler:
	//
	// if isDevelopment {
	//		reload.directories = []string{"ui/"}
	//		handler = reload.Handle(handler)
	//}
	directories []string
	// Endpoint defines what path the WebSocket connection is formed over.
	// It is set to "/reload_ws" by default.
	Endpoint string

	Log *log.Logger

	// Used to upgrade connections to Websocket connections
	Upgrader websocket.Upgrader

	// used to reload all websocket connections at once
	cond sync.Cond
}

func New(directories ...string) *Reloader {
	r := &Reloader{
		directories: directories,
		Endpoint:    "/reload_ws",
		Log:         log.New(os.Stdout, "Reload: ", log.Lmsgprefix|log.Ltime),
		cond:        *sync.NewCond(&sync.Mutex{}),
		Upgrader:    websocket.Upgrader{},
	}
	go r.WatchDirectories()
	return r
}

// Handle starts the reload middleware, watching the specified `Directories`.
// This middleware should only be called once, at the top of the middleware chain.
func (reload *Reloader) Handle(next http.Handler) http.Handler {
	scriptToInject := InjectedScript(reload.Endpoint)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Endpoint == "/reload_ws" by default
		if r.URL.Path == reload.Endpoint {
			reload.ServeWS(w, r)
			return
		}
		// set headers first so that they're sent with the initial response
		{
			// disable caching, because http.FileServer will
			// send Last-Modified headers, prompting the browser to cache it
			w.Header().Set("Cache-Control", "no-cache")
		}

		body := &bytes.Buffer{}
		wrap := newWrapResponseWriter(w, r.ProtoMajor)
		// copy body so that we can sniff the content type
		wrap.Tee(body)

		next.ServeHTTP(wrap, r)
		contentType := w.Header().Get("Content-Type")

		if contentType == "" {
			contentType = http.DetectContentType(body.Bytes())
		}

		if strings.HasPrefix(contentType, "text/html") {
			// just append the script to the end of the document
			// this is invalid HTML, but browsers will accept it anyways
			// should be fine for development purposes
			w.Write([]byte(scriptToInject))
		}
	})
}

// ServeWS is the default websocket endpoint.
// Implementing your own is easy enough if you
// don't want to use 'gorilla/websocket'
func (reload *Reloader) ServeWS(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("v")
	if version != wsCurrentVersion {
		reload.Log.Printf("Injected script version is out of date (v%s < v%s)\n", version, wsCurrentVersion)
	}

	conn, err := reload.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		reload.Log.Printf("ServeWS error: %s\n", err)
		return
	}

	// Block here until next reload event
	reload.Wait()

	conn.WriteMessage(websocket.TextMessage, []byte("reload"))
	conn.Close()
}

func (reload *Reloader) Wait() {
	reload.cond.L.Lock()
	reload.cond.Wait()
	reload.cond.L.Unlock()
}

// Returns the javascript that will be injected on each HTML page.
func InjectedScript(endpoint string) string {
	return fmt.Sprintf(`
<script>
	function retry() {
	  setTimeout(() => listen(true), 1000)
	}
	function listen(isRetry) {
	  let ws = new WebSocket("ws://" + location.host + "%s?v=%s")
	  if(isRetry) {
	    ws.onopen = () => window.location.reload()
	  }
	  ws.onmessage = function(msg) {
	    if(msg.data === "reload") {
	      window.location.reload()
	    }
	  }
	  ws.onclose = retry
	}
	listen(false)
</script>`, endpoint, wsCurrentVersion)
}
