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
	md = "md2html"
)

func md2html(path, wikiDir string) error {
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

func buildWiki(srcDir, wikiDir string) error {
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

			err := md2html(path, wikiDir)
			if err != nil {
				log.Println("Could not parse file", err)
			}
		}(path)

		return nil
	}

	err := filepath.WalkDir(srcDir, walkFunc)
	if err != nil {
		return err
	}

	wg.Wait()

	return nil
}

func fileWatcher(srcDir string, fileChanged chan<- string) error {
	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		go func(path string) {
			initialStat, err := os.Stat(path)
			if err != nil {
				log.Println("Could not get initial file info", err)
			}

			for {
				time.Sleep(time.Second)

				stat, err := os.Stat(path)
				if err != nil {
					log.Println("Could not get file info", err)
				}

				if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
					fileChanged <- path
					initialStat = stat
				}
			}
		}(path)

		return nil
	}

	filepath.WalkDir(srcDir, walkFunc)

	return nil
}

func main() {
	var srcDir string
	if len(os.Args) == 2 {
		srcDir = os.Args[1]
	} else {
		srcDir = "."
	}

	_, err := exec.LookPath(md)
	if err != nil {
		log.Fatal(err)
	}

	wikiDir, err := os.MkdirTemp("", srcDir)
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(wikiDir)

	err = buildWiki(srcDir, wikiDir)
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

	fileChanged := make(chan string)

	go func() {
		log.Println("Starting file watcher")

		err = fileWatcher(srcDir, fileChanged)
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		for {
			path := <-fileChanged

			log.Println("Recompiling ", path)

			err := md2html(path, wikiDir)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	// Block here until Interrupt is received
	<-c
	log.Println("Exiting")
}
