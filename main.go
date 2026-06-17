package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	url := "http://localhost:" + port

	http.HandleFunc("/api/wrapped", handleWrapped)
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/timeline", handleTimeline)
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	fmt.Println("cc-lens Agent Wrapped -> " + url)
	go openBrowser(url)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleWrapped(w http.ResponseWriter, r *http.Request) {
	wrapped, err := BuildWrapped()
	writeJSON(w, wrapped, err)
}

func handleTimeline(w http.ResponseWriter, r *http.Request) {
	tl, err := ParseTimeline()
	writeJSON(w, tl, err)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := ParseHistory()
	writeJSON(w, stats, err)
}

func writeJSON(w http.ResponseWriter, value any, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

func openBrowser(url string) {
	if os.Getenv("CC_LENS_NO_BROWSER") == "1" {
		return
	}
	time.Sleep(250 * time.Millisecond)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
