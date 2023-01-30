// Exposes a singleton which can be used to hot-reload
// Go templates in development.
//
// Typically, integrating this package looks like this:
//
// 1. Add the paths where your html/js/css is contained:
//
//	reload.Paths = []string{"ui/"}
//
// 2. Expose the WS endpoint:
//
//	http.HandleFunc("/reload", reload.ServeWS)
//
// 3. Inject the JS into your template:
//
//	data := map[string]any {
//		LiveReload: reload.InjectedScript("/reload"),
//	}
//	templateCache.ExecuteTemplate(w, "index.html", data)
//
// 4. Insert the script into the main template's <body>:
//
//	{{ .LiveReload }}
//
// 5. Use the reloader.OnReload callback to re-parse the templates
// if they are cached
//
//	reload.OnReload = func() {
//		templateCache = newTemplateCache()
//	}
//
// See the full example at https://github.com/aarol/reload/example/main.go
package reload

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

var (
	Paths    = []string{}
	OnReload func()
	Logger   = log.New(os.Stdout, "Reload: ", log.Lmsgprefix|log.Ltime)
	upgrader = &websocket.Upgrader{}
	cond     = sync.NewCond(&sync.Mutex{})
)

// Run listens for changes in directories and
// broadcasts on write.
//
// Run initalizes the watcher and should only be called once in a separate goroutine.
func Run() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Logger.Printf("error initializing fsnotify watcher: %s\n", err)
	}

	for _, path := range Paths {
		directories, err := recursiveWalk(path)
		if err != nil {
			Logger.Printf("Error walking directories: %s\n", err)
			return
		}
		for _, dir := range directories {
			watcher.Add(dir)
		}
	}

	Logger.Println("Watching", strings.Join(Paths, ","), "for changes")
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

func reloadDedup(w *fsnotify.Watcher) {
	wait := 100 * time.Millisecond

	lastEdited := ""

	timer := time.AfterFunc(wait, func() {
		Logger.Println("Edit", lastEdited)
		if OnReload != nil {
			OnReload()
		}
		cond.Broadcast()
	})
	timer.Stop()

	defer w.Close()

	for {
		select {
		case err, ok := <-w.Errors:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			Logger.Println("error watching: ", err)
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			switch {
			case e.Has(fsnotify.Create):
				// Watch any created file/directory
				if err := w.Add(e.Name); err != nil {
					log.Printf("error watching %s: %s\n", e.Name, err)
				}
				lastEdited = path.Base(e.Name)
				timer.Reset(wait)

			case e.Has(fsnotify.Write):
				lastEdited = path.Base(e.Name)
				timer.Reset(wait)

			case e.Has(fsnotify.Rename):
				// a renamed file might be outside
				// of the specified paths
				directories, _ := recursiveWalk(e.Name)
				for _, v := range directories {
					w.Remove(v)
				}
				w.Remove(e.Name)

			case e.Has(fsnotify.Remove):
				directories, _ := recursiveWalk(e.Name)
				for _, v := range directories {
					w.Remove(v)
				}
				w.Remove(e.Name)
			}
		}
	}
}

// Returns the Javascript that should be embedded into the site as template.HTML.
//
// The browser will listen to a websocket connection at "ws://<address>/<endpoint>".
//
// Example:
//
//	reload.InjectedScript("/reload")
func InjectedScript(endpoint string) template.HTML {
	endpoint = strings.TrimLeftFunc(endpoint, func(r rune) bool { return r == '/' })
	return template.HTML(fmt.Sprintf(
		`<script>
		function listen(isRetry) {
			let ws = new WebSocket("ws://" + location.host + "/%s")
			if(isRetry) {
				ws.onopen = () => window.location.reload()
			}
			ws.onmessage = function(ev) {
				if(ev.data === "reload") {
					window.location.reload()
				}
			}
			ws.onerror = function(ev) {
				setTimeout(() => listen(true), 2000);
			}
		}
		listen(false)
		</script>`, endpoint))
}

func recursiveWalk(path string) ([]string, error) {
	res := []string{}
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			res = append(res, path)
		}
		return nil
	})

	return res, err
}
