package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const developerTag = `
     ██▀███   ▄▄▄      ▄▄▄██▀▀▀▄▄▄       ██▀███  ▓█████ 
    ▓██ ▒ ██▒▒████▄      ▒██  ▒████▄    ▓██ ▒ ██▒▓█   ▀ 
    ▓██ ░▄█ ▒▒██  ▀█▄    ░██  ▒██  ▀█▄  ▓██ ░▄█ ▒▒███   
    ▒██▀▀█▄  ░██▄▄▄▄██▓██▄██▓ ░██▄▄▄▄██ ▒██▀▀█▄  ▒▓█  ▄ 
    ░██▓ ▒██▒ ▓█   ▓██▒▓███▒   ▓█   ▓██▒░██▓ ▒██▒░▒████▒
    ░ ▒▓ ░▒▓░ ▒▒   ▓▒█░▒▓▒▒░   ▒▒   ▓▒█░░ ▒▓ ░▒▓░░░ ▒░ ░
      ░▒ ░ ▒░   ▒   ▒▒ ░▒ ░▒░    ▒   ▒▒ ░   ░▒ ░ ▒░ ░ ░  ░
      ░░   ░   ░   ▒   ░ ░ ░    ░   ▒     ░░   ░    ░   
       ░           ░  ░░   ░        ░  ░   ░        ░  ░
    `
const CurrentVersion = "1.0.2"

//go:embed ui
var uiFiles embed.FS

// URL vers un fichier JSON sur GitHub ou ton serveur
const UpdateConfigURL = "https://raw.githubusercontent.com/RajareCorp/Xaladownloader/master/update.json"

// --- Logique App ---

func getEpisodes(ctx context.Context, mediaID int, seasonNum int) ([]Episode, error) {
	remote := fmt.Sprintf("%s/api/v1/media/%d/season/%d", BaseURL, mediaID, seasonNum)
	req, _ := http.NewRequestWithContext(ctx, "GET", remote, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data SeasonDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Data.Items.Episodes, nil
}

func sanitizeFileName(name string) string {
	return regexp.MustCompile(`[\\/:*?"<>|]`).ReplaceAllString(name, "")
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func main() {
	fmt.Println(developerTag)
	fmt.Printf("Version actuelle: %s\n", CurrentVersion)

	// Vérifier les mises à jour en arrière-plan ou au démarrage
	CheckForUpdates()
	InitApp()

	// On extrait le sous-dossier "ui"
	strippedFS, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		log.Fatal(err)
	}

	// On crée le FileServer
	fileServer := http.FileServer(http.FS(strippedFS))

	// Handler pour servir les fichiers avec les bons types MIME
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Fix pour le MIME type JavaScript/CSS si Go fait une erreur
		if strings.HasSuffix(path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(path, ".css") {
			w.Header().Set("Content-Type", "text/css")
		}
		fileServer.ServeHTTP(w, r)
	})
	http.HandleFunc("/api/search", searchHandler)
	http.HandleFunc("/api/episodes", episodesHandler)
	http.HandleFunc("/api/download", downloadHandler)
	http.HandleFunc("/api/last-releases", lastReleasesHandler)
	http.HandleFunc("/api/franchise", franchiseHandler)

	go func() {
		fmt.Println("Démarrage sur http://127.0.0.1:8080")
		http.ListenAndServe(":8080", nil)
	}()

	time.Sleep(500 * time.Millisecond)
	openBrowser("http://127.0.0.1:8080")
	select {}
}
