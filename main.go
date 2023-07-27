package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
				log.Println(err.Error())
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

func main() {
	_, err := exec.LookPath(md)
	if err != nil {
		log.Fatal(err)
	}

	err = buildWiki(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	log.Println("wiki built!")
	log.Println("Starting server on :1234")

	err = http.ListenAndServe(":1234", http.FileServer(http.Dir(wikiDir)))
	if err != nil {
		log.Fatal(err)
	}
}
