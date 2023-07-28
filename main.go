package main

import (
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type TemplateData struct {
	Title   string
	Content template.HTML
}

const markdownCmd = "md2html"

var (
	//go:embed template.html
	htmlTemplateFile string
	htmlTemplate     *template.Template
	wikiDir          string
	srcDir           = flag.String("f", ".", "markdown src dir")
)

func md2html(path string) error {
	// Compile markdown to html
	bytes, err := exec.Command(markdownCmd, "--github", path).Output()
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
	withDir := filepath.Join(wikiDir, fmt.Sprintf("%s.html", withoutExt))

	// Create file
	file, err := os.Create(withDir)
	if err != nil {
		return err
	}
	defer file.Close()

	data := TemplateData{
		Title:   strings.Title(filepath.Base(withoutExt)),
		Content: template.HTML(bytes),
	}
	// Write HTML content into file
	err = htmlTemplate.Execute(file, data)
	if err != nil {
		return err
	}

	return nil
}

func buildWiki() error {
	g := new(errgroup.Group)

	walkFunc := func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		// Launch a goroutine for each the building of each markdown file
		g.Go(func() error {
			err := md2html(path)
			if err != nil {
				return err
			}

			return nil
		})

		return nil
	}

	err := filepath.WalkDir(*srcDir, walkFunc)
	if err != nil {
		return err
	}

	// Wait for all files to build
	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func fileWatcher(path string, fileChanged chan<- string) {
	// Fetch initial stat as a comparison value
	initialStat, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wiki: could not get file info: %v", err)
	}

	for {
		time.Sleep(time.Second)

		// Every one second, fetch stat again
		stat, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wiki: could not get file info: %v", err)
		}

		// If file has changed, send path to channel and replace initialStat
		if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
			fileChanged <- path
			initialStat = stat
		}
	}
}

func launchFileWatchers() error {
	fileChanged := make(chan string)

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		// Launch a goroutine to watch each markdown file
		go fileWatcher(path, fileChanged)

		return nil
	}

	err := filepath.WalkDir(*srcDir, walkFunc)
	if err != nil {
		return err
	}

	// Poll for events: recompile if file changed
	go func() {
		for {
			path := <-fileChanged

			fmt.Println("Recompiling ", path)

			err := md2html(path)
			if err != nil {
				fmt.Println(err)
			}
		}
	}()

	return nil
}

func serveAndWatchFiles() {
	// Launch file server
	go func() {
		fmt.Println("Starting server on :1234")

		err := http.ListenAndServe(":1234", http.FileServer(http.Dir(wikiDir)))
		if err != nil {
			fmt.Println(err)
		}
	}()

	// Launch file watcher
	go func() {
		fmt.Println("Starting file watcher")

		err := launchFileWatchers()
		if err != nil {
			fmt.Println(err)
		}
	}()
}

func run() error {
	// Check for markdown executable
	if _, err := exec.LookPath(markdownCmd); err != nil {
		return err
	}

	// Create temporary dir
	tempDir, err := os.MkdirTemp("", *srcDir)
	if err != nil {
		return err
	}
	wikiDir = tempDir
	defer os.RemoveAll(wikiDir)

	// Parse template for markdown
	htmlTemplate, err = template.New("markdown").Parse(htmlTemplateFile)
	if err != nil {
		return err
	}

	// Go through each md file in srcDir, build it and create an HTML file in
	// wikiDir
	err = buildWiki()
	if err != nil {
		return err
	}
	fmt.Println("wiki built!")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Launch file server and file watcher
	serveAndWatchFiles()

	// Block here until Interrupt is received
	<-c
	fmt.Println()
	fmt.Println("Exiting")

	return nil
}

func main() {
	flag.Parse()

	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "wiki: %v\n", err)
		os.Exit(1)
	}
}
