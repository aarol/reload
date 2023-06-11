package reload

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchDirectories listens for changes in directories and
// broadcasts on write.
func WatchDirectories(directories []string) {
	if len(directories) == 0 {
		Log.Println("no directories provided (reload.Directories is empty)")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Log.Printf("error initializing fsnotify watcher: %s\n", err)
	}

	for _, path := range directories {
		directories, err := recursiveWalk(path)
		if err != nil {
			fmt.Printf("%T\n", err)
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				Log.Printf("directory doesn't exist: %s", pathErr.Path)
			} else {
				Log.Printf("error walking directories: %s\n", err)
			}
			return
		}
		for _, dir := range directories {
			watcher.Add(dir)
		}
	}

	Log.Println("watching", strings.Join(directories, ","), "for changes")
	reloadDedup(watcher)
}

func reloadDedup(w *fsnotify.Watcher) {
	wait := 100 * time.Millisecond

	lastEdited := ""

	timer := time.AfterFunc(wait, func() {
		Log.Println("Edit", lastEdited)
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
			Log.Println("error watching: ", err)
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
				// a renamed file might be outside the specified paths
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
	var res []string
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
