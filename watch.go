package reload

import (
	"errors"
	"io/fs"
	"log"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
)

// WatchDirectories listens for changes in directories and
// broadcasts on write.
func (reload *Reloader) WatchDirectories() {
	if len(reload.directories) == 0 {
		reload.Log.Println("no directories provided (reload.Directories is empty)")
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		reload.Log.Printf("error initializing fsnotify watcher: %s\n", err)
	}

	for _, path := range reload.directories {
		directories, err := recursiveWalk(path)
		if err != nil {
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				reload.Log.Printf("directory doesn't exist: %s\n", pathErr.Path)
			} else {
				reload.Log.Printf("error walking directories: %s\n", err)
			}
			return
		}
		for _, dir := range directories {
			w.Add(dir)
		}
	}

	reload.Log.Println("watching", strings.Join(reload.directories, ","), "for changes")

	debounce := debounce.New(100 * time.Millisecond)

	callback := func(path string) func() {
		return func() {
			reload.Log.Println("Edit", path)
			if reload.OnReload != nil {
				reload.OnReload()
			}
			reload.cond.Broadcast()
		}
	}

	defer w.Close()

	for {
		select {
		case err := <-w.Errors:
			reload.Log.Println("error watching: ", err)
		case e := <-w.Events:
			switch {
			case e.Has(fsnotify.Create):
				// Watch any created file/directory
				if err := w.Add(e.Name); err != nil {
					log.Printf("error watching %s: %s\n", e.Name, err)
				}
				debounce(callback(path.Base(e.Name)))

			case e.Has(fsnotify.Write):
				debounce(callback(path.Base(e.Name)))

			case e.Has(fsnotify.Rename), e.Has(fsnotify.Remove):
				// a renamed file might be outside the specified paths
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
