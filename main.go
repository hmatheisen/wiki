package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	wikiDir = "wikiFiles"
	md      = "md2html"
)

func md2html(path string) error {
	bytes, err := exec.Command(md, "--github", path).Output()
	if err != nil {
		return err
	}

	// Create folder if needed
	dirPath := filepath.Dir(filepath.Join(wikiDir, path))
	if _, err = os.Stat(dirPath); os.IsNotExist(err) {
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			return err
		}
	}

	withoutExt := strings.TrimSuffix(path, filepath.Ext(path))
	withHTML := fmt.Sprintf("%s.html", withoutExt)
	withDir := filepath.Join(wikiDir, withHTML)

	err = os.WriteFile(withDir, bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

func buildWiki(dir string) error {
	var wg sync.WaitGroup

	walkFunc := func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		wg.Add(1)

		go func(path string) {
			defer wg.Done()

			err := md2html(path)
			if err != nil {
				log.Printf("Could not parse file %v\n", err)
			}
		}(path)

		return nil
	}

	err := filepath.WalkDir(dir, walkFunc)
	if err != nil {
		return err
	}

	wg.Wait()

	return nil
}

func fileWatcher(dir string) error {
	fileChanged := make(chan string)

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		go func(path string) {
			initialStat, err := os.Stat(path)
			if err != nil {
				// FIXME
				log.Println(err)
			}

			for {
				time.Sleep(time.Second)

				stat, err := os.Stat(path)
				if err != nil {
					// FIXME
					log.Println(err)
				}

				if stat.Size() != initialStat.Size() ||
					stat.ModTime() != initialStat.ModTime() {
					fileChanged <- path
					initialStat = stat
				}
			}
		}(path)

		return nil
	}

	filepath.WalkDir(dir, walkFunc)

	go func() {
		for {
			path := <- fileChanged

			log.Println("Recompiling   ", path)

			err := md2html(path)
			if err != nil {
				// FIXME
				log.Println(err)
			}
		}
	}()

	return nil
}

func main() {
	dir := os.Args[1]

	_, err := exec.LookPath(md)
	if err != nil {
		log.Fatal(err)
	}

	err = buildWiki(dir)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("wiki built!")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		log.Println("Starting server on :1234")

		err = http.ListenAndServe(":1234", http.FileServer(http.Dir(wikiDir)))
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		log.Println("Starting file watcher")

		err = fileWatcher(dir)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Block here until Interrupt is received
	<-c
	log.Println("Exiting")
}
