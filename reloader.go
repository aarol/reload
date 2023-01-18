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

var upgrader = &websocket.Upgrader{}

type Reloader struct {
	// Disabled should be set to true in production environments.
	Disabled bool
	// Directories to watch recursively.
	// Usually just a []string{"public"}
	Paths []string
	// OnReload will be called before every reload.
	// If you have a template cache, you should regenerate it
	// with this function.
	OnReload func()
	// Where the client Websocket should connect to.
	// This will be used in InjectedScript()
	//
	// Recommended value: "/reload"
	EndpointPath string
	Logger       *log.Logger
	cond         *sync.Cond
}

// Run listens for changes in directories and
// broadcasts on write.
//
// Run initalizes the watcher and should only be called once in a separate goroutine.
func (r *Reloader) Run() {
	if r.Disabled {
		return
	}

	r.cond = sync.NewCond(&sync.Mutex{})

	r.Logger = log.New(os.Stdout, "Reload: ", log.Lmsgprefix|log.Ltime)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	go r.reloadDedup(watcher)

	for _, path := range r.Paths {
		directories, err := recursiveWalk(path)
		if err != nil {
			r.Logger.Printf("Error walking directories: %s\n", err)
			return
		}
		for _, dir := range directories {
			watcher.Add(dir)
		}
	}

	r.Logger.Println("Watching", strings.Join(r.Paths, ","), "for changes")
}

func (r *Reloader) Wait() {
	r.cond.L.Lock()
	r.cond.Wait()
	r.cond.L.Unlock()
}

// The default websocket endpoint.
// Implementing your own is easy enough if you
// don't want to use 'gorilla/websocket'
func (r *Reloader) ServeWS(w http.ResponseWriter, req *http.Request) {
	if r.Disabled {
		http.NotFound(w, req)
		return
	}
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		r.Logger.Println(err)
		return
	}

	// Block here until next reload event
	r.Wait()

	conn.WriteMessage(websocket.TextMessage, []byte("reload"))
	conn.Close()
}

func (r *Reloader) reloadDedup(w *fsnotify.Watcher) {
	wait := 100 * time.Millisecond

	lastEdited := ""

	timer := time.AfterFunc(wait, func() {
		r.Logger.Println("Edit", lastEdited)
		r.cond.Broadcast()
	})
	timer.Stop()

	defer w.Close()

	for {
		select {
		case err, ok := <-w.Errors:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			r.Logger.Println("error watching: ", err)
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			fmt.Println(e)
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

// Returns the Javascript that should be embedded into the site.
//
// The browser will listen to a websocket connection at "ws://<address>/reload".
//
// Will return an empty string when 'Disabled' is true
func (r *Reloader) InjectedScript() template.HTML {
	if r.Disabled {
		return ""
	}
	return template.HTML(fmt.Sprintf(
		`<script>
		function listen(isRetry) {
			let ws = new WebSocket("ws://" + location.host + "%s")
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
		</script>`, r.EndpointPath))
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
