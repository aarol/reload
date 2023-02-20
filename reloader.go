// Exposes a singleton which can be used to trigger a reload
// in the browser whenever a file is changed.
//
// Reload doesn't require any external tools and is can be
// integrated into any project that uses the standard net/http interface.
//
// Typically, integrating this package looks like this:
//
// 1. Insert the WatchAndInject() middleware at the top of the request chain
//
//	var handler http.Handler = http.DefaultServeMux
//
//	if isDevelopment {
//		handler = reload.WatchAndInject("ui/")(handler)
//	}
//
//	http.ListenAndServe("localhost:3001", handler)
//
// 2. (Optional) Use the reloader.OnReload callback to re-parse any cached templates
//
//	reload.OnReload = func() {
//		app.templateCache = newTemplateCache()
//	}
//
// If the built-in http.Handler middleware doesn't work for you,
// you can still use the `ServeWS()`, `InjectScript()`, `Wait()` and WatchDirectories() functions manually.
//
// See the full example at https://github.com/aarol/reload/example/main.go
package reload

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

var (
	OnReload      func()
	Logger        = log.New(os.Stdout, "Reload: ", log.Lmsgprefix|log.Ltime)
	upgrader      = &websocket.Upgrader{}
	cond          = sync.NewCond(&sync.Mutex{})
	defaultInject = InjectedScript("/reload")
)

//go:embed error.html
var errorHTML string

// WatchDirectories listens for changes in directories and
// broadcasts on write.
//
// WatchDirectories initalizes the watcher and should only be called once in separate new goroutine.
func WatchDirectories(directories []string) {
	if len(directories) == 0 {
		Logger.Println("No directories specified; returning")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Logger.Printf("error initializing fsnotify watcher: %s\n", err)
	}

	for _, path := range directories {
		directories, err := recursiveWalk(path)
		if err != nil {
			Logger.Printf("Error walking directories: %s\n", err)
			return
		}
		for _, dir := range directories {
			watcher.Add(dir)
		}
	}

	Logger.Println("Watching", strings.Join(directories, ","), "for changes")
	reloadDedup(watcher)
}

func Wait() {
	cond.L.Lock()
	cond.Wait()
	cond.L.Unlock()
}

// The default websocket endpoint.
// Implementing your own is easy enough if you
// don't want to use 'gorilla/websocket'
func ServeWS(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		Logger.Println(err)
		return
	}

	// Block here until next reload event
	Wait()

	conn.WriteMessage(websocket.TextMessage, []byte("reload"))
	conn.Close()
}

func WatchAndInject(directoriesToWatch ...string) func(next http.Handler) http.Handler {
	go WatchDirectories(directoriesToWatch)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/reload" {
				ServeWS(w, r)
				return
			}

			wrap := &wrapper{ResponseWriter: w, buf: &bytes.Buffer{}}
			next.ServeHTTP(wrap, r)

			body := wrap.buf.Bytes()
			contentType := w.Header().Get("Content-Type")

			if contentType == "" {
				contentType = http.DetectContentType(body)
			}

			switch {
			case strings.HasPrefix(contentType, "text/html"):
				body = insertScriptIntoHTML(body, []byte(defaultInject))

			case wrap.header >= 400 && strings.HasPrefix(contentType, "text/plain"):
				buf := &bytes.Buffer{}
				fmt.Fprintf(buf, errorHTML, defaultInject, http.StatusText(wrap.header), body)
				body = buf.Bytes()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			}
			if wrap.header != 0 {
				w.WriteHeader(wrap.header)
			}

			w.Write(body)
		})
	}
}

// Returns the Javascript that should be embedded into the site as template.HTML.
//
// The browser will listen to a websocket connection at "ws://<address>/<endpoint>".
//
// Example:
//
//	template.HTML(reload.InjectedScript("/reload"))
func InjectedScript(endpoint string) string {
	return fmt.Sprintf(
		`<script>
		function listen(isRetry) {
			let ws = new WebSocket("ws://" + location.host + "%s")
			if(isRetry) {
				ws.onopen = () => window.location.reload()
			}
			ws.onmessage = function(msg) {
				if(msg.data === "reload") {
					window.location.reload()
				}
			}
			ws.onerror = function(ev) {
				setTimeout(() => listen(true), 1000);
			}
		}
		listen(false)
		</script>`, endpoint)
}
