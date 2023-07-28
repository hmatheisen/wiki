package main

import (
	"fmt"
	"io/fs"
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

var (
	srcDir = "."
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
				fmt.Println("Could not parse file", err)
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
				fmt.Println("Could not get initial file info", err)
			}

			for {
				time.Sleep(time.Second)

				stat, err := os.Stat(path)
				if err != nil {
					fmt.Println("Could not get file info", err)
				}

				if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
					fileChanged <- path
					initialStat = stat
				}
			}
		}(path)

		return nil
	}

	err := filepath.WalkDir(srcDir, walkFunc)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	if len(os.Args) == 2 {
		srcDir = os.Args[1]

		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Dir %s does not exists\n", srcDir)
			os.Exit(2)
		}
	}

	_, err := exec.LookPath(md)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func launch(wikiDir string) {
	// Launch file server
	go func() {
		fmt.Println("Starting server on :1234")

		err := http.ListenAndServe(":1234", http.FileServer(http.Dir(wikiDir)))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	fileChanged := make(chan string)

	// Launch file fatcher
	go func() {
		fmt.Println("Starting file watcher")

		err := fileWatcher(srcDir, fileChanged)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	// Poll for events: recompile if file changed
	go func() {
		for {
			path := <-fileChanged

			fmt.Println("Recompiling ", path)

			err := md2html(path, wikiDir)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}()
}

func main() {
	wikiDir, err := os.MkdirTemp("", srcDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer os.RemoveAll(wikiDir)

	err = buildWiki(srcDir, wikiDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("wiki built!")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	launch(wikiDir)

	// Block here until Interrupt is received
	<-c
	fmt.Println()
	fmt.Println("Exiting")
}
