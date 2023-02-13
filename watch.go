package reload

import (
	"io/fs"
	"log"
	"path"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

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
				timer.Reset(wait)
				lastEdited = path.Base(e.Name)

			case e.Has(fsnotify.Write):
				timer.Reset(wait)
				lastEdited = path.Base(e.Name)

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
