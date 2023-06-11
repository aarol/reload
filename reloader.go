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
//		handler = reload.WatchAndInject("ui/")(handler)
//	}
//
//	http.ListenAndServe("localhost:3001", handler)
//
// 2. Use the reloader.OnReload callback to re-parse any cached templates (Optional)
//
//	reload.OnReload = func() {
//		app.templateCache = parseTemplates()
//	}
//
// The package also exposes `ServeWS`, `InjectScript`, `Wait` and `WatchDirectories`,
// which can be used to embed the script in the templates directly.
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

	"github.com/gorilla/websocket"
)

const wsCurrentVersion = "1"

var (
	// OnReload will be called after a file changes, but before the browser reloads.
	OnReload func()
	// A slice of directories that will be recursively watched for changes.
	// This should always be set before creating the handler:
	//
	// if isDevelopment {
	//		reload.Directories = []string{"ui/"}
	//		handler = reload.Handle(handler)
	//}
	Directories []string
	Log         = log.New(os.Stdout, "Reload: ", log.Lmsgprefix|log.Ltime)

	upgrader = &websocket.Upgrader{}
	cond     = sync.NewCond(&sync.Mutex{})

	defaultInject = InjectedScript("/reload")
)

// Handle starts the reload middleware, watching the directories provided by `reload.Directories`
// This middleware should only be called once, at the top of the middleware chain.
func Handle(next http.Handler) http.Handler {
	go WatchDirectories(Directories)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reload" {
			ServeWS(w, r)
			return
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
			// this should be fine for development purposes
			w.Write([]byte(defaultInject))
		}
	})
}

// ServeWS is the default websocket endpoint.
// Implementing your own is easy enough if you
// don't want to use 'gorilla/websocket'
func ServeWS(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("v")
	if version != wsCurrentVersion {
		Log.Printf("Injected script version is out of date (v%s < v%s)\n", version, wsCurrentVersion)
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Log.Println(err)
		return
	}

	// Block here until next reload event
	Wait()

	conn.WriteMessage(websocket.TextMessage, []byte("reload"))
	conn.Close()
}

func Wait() {
	cond.L.Lock()
	cond.Wait()
	cond.L.Unlock()
}

// Returns the javascript that will be
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
