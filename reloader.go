package reload

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

var (
	upgrader = &websocket.Upgrader{}
)

type Reloader struct {
	// If Disabled is true, the whole package will be short-circuited
	Disabled bool
	// Directories to watch recursively.
	// Usually just a []string{"public"}
	Paths []string
	// OnReload will be called before every reload.
	// If you have a template cache, you should regenerate it
	// with this function.
	OnReload func()
	cond     *sync.Cond
}

// Run listens for changes in directories and
// broadcasts on write.
func (r *Reloader) Run() {
	if r.Disabled {
		return
	}

	r.cond = sync.NewCond(&sync.Mutex{})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	notify := make(chan struct{})
	go reloadDedup(watcher, notify)

	go func() {
		for {
			<-notify
			if r.OnReload != nil {
				r.OnReload()
			}
			r.cond.Broadcast()
		}
	}()

	for _, path := range r.Paths {
		directories, err := recursiveWalk(path)
		if err != nil {
			log.Panicf("Error walking directories: %s", err)
		}
		for _, dir := range directories {
			watcher.Add(dir)
		}
	}

	log.Println("Watching", strings.Join(r.Paths, ","), "for changes")
}

func (r *Reloader) Wait() {
	r.cond.L.Lock()
	r.cond.Wait()
	r.cond.L.Unlock()
}

// Uses gorilla/websocket under the hood.
func (r *Reloader) ServeWS(w http.ResponseWriter, req *http.Request) {
	if r.Disabled {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
		return
	}
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Println(err)
		return
	}

	r.Wait()

	conn.WriteMessage(websocket.TextMessage, []byte("reload"))

	conn.Close()
}

func reloadDedup(w *fsnotify.Watcher, notify chan struct{}) {

	wait := 100 * time.Millisecond

	lastEdited := ""

	timer := time.AfterFunc(wait, func() {
		notify <- struct{}{}
		log.Println("Edit:", lastEdited)
	})
	timer.Stop()

	defer close(notify)

	for {
		select {
		case err, ok := <-w.Errors:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			log.Println("error watching: ", err)
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			switch {
			case e.Has(fsnotify.Create):
				if err := w.Add(e.Name); err != nil {
					log.Printf("error watching %s: %s\n", e.Name, err)
				}
				lastEdited = path.Base(e.Name)
				timer.Reset(wait)

			case e.Has(fsnotify.Write):
				lastEdited = path.Base(e.Name)
				timer.Reset(wait)

			case e.Has(fsnotify.Rename):
				w.Remove(e.Name)
			}
		}
	}
}

// Returns the Javascript that should be embedded into the site
// The browser will listen to a websocket connection at "/reload".
//
// Will return an empty string when 'Disabled' is true
func (r *Reloader) InjectedScript() template.HTML {
	if r.Disabled {
		return ""
	}
	return `
		<script>
		function listen(isRetry) {
			let ws = new WebSocket("ws://" + location.host + "/reload")
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
		</script>`
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
