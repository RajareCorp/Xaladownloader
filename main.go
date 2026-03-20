package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
const CurrentVersion = "1.0.3"

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

// DownloadM3U8 avec gestion de buffer large et retries
func DownloadM3U8(targetURL string, fileName string) error {
	finalURL, segments, err := resolveM3U8(targetURL)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	downloadPath := filepath.Join(home, "Downloads", sanitizeFileName(fileName)+".mp4")

	finalFile, err := os.Create(downloadPath)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	total := len(segments)
	// Augmentation du timeout et ajout d'un transport personnalisé
	client := &http.Client{
		Timeout: 45 * time.Second,
	}
	baseURL, _ := url.Parse(finalURL)

	for i, segLine := range segments {
		m3u8Progress[fileName] = fmt.Sprintf("Téléchargement : %d/%d segments", i+1, total)
		fmt.Printf("\rProgression : %d/%d", i+1, total)

		u, _ := url.Parse(segLine)
		segmentURL := baseURL.ResolveReference(u).String()

		// Tentative de téléchargement avec 3 essais en cas d'échec
		var segResp *http.Response
		for retry := 0; retry < 3; retry++ {
			req, _ := http.NewRequest("GET", segmentURL, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

			segResp, err = client.Do(req)
			if err == nil && segResp.StatusCode == 200 {
				break
			}
			time.Sleep(1 * time.Second) // Pause avant retry
		}

		if err != nil || segResp == nil || segResp.StatusCode != 200 {
			log.Printf("\n[!] Échec définitif du segment %d", i)
			continue
		}

		_, err = io.Copy(finalFile, segResp.Body)
		segResp.Body.Close()
		if err != nil {
			return err
		}
	}

	finalFile.Sync()
	m3u8Progress[fileName] = "Terminé ! (Vérifiez vos Téléchargements)"
	return nil
}

// resolveM3U8 avec un buffer illimité pour les playlists géantes
func resolveM3U8(uri string) (string, []string, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	var segments []string
	baseURL, _ := url.Parse(uri)

	// Utilisation de bufio.Reader au lieu de Scanner pour éviter la limite de ligne
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.Contains(line, ".m3u8") {
				nextURL := baseURL.ResolveReference(&url.URL{Path: line}).String()
				return resolveM3U8(nextURL)
			}
			segments = append(segments, line)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return "", nil, err
		}
	}

	if len(segments) == 0 {
		return "", nil, fmt.Errorf("aucune donnée trouvée")
	}
	return uri, segments, nil
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
	http.HandleFunc("/api/check-url", checkURLHandler)
	http.HandleFunc("/api/m3u8-download", m3u8Handler)
	http.HandleFunc("/api/m3u8-status", m3u8StatusHandler)

	go func() {
		fmt.Println("Démarrage sur http://127.0.0.1:8080")
		http.ListenAndServe(":8080", nil)
	}()

	time.Sleep(500 * time.Millisecond)
	openBrowser("http://127.0.0.1:8080")
	select {}
}
