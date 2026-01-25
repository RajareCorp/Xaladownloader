package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Tag du développeur
const developerTag = `
     ██▀███   ▄▄▄      ▄▄▄██▀▀▀▄▄▄       ██▀███  ▓█████ 
    ▓██ ▒ ██▒▒████▄      ▒██  ▒████▄    ▓██ ▒ ██▒▓█   ▀ 
    ▓██ ░▄█ ▒▒██  ▀█▄    ░██  ▒██  ▀█▄  ▓██ ░▄█ ▒▒███   
    ▒██▀▀█▄  ░██▄▄▄▄██▓██▄██▓ ░██▄▄▄▄██ ▒██▀▀█▄  ▒▓█  ▄ 
    ░██▓ ▒██▒ ▓█   ▓██▒▓███▒   ▓█   ▓██▒░██▓ ▒██▒░▒████▒
    ░ ▒▓ ░▒▓░ ▒▒   ▓▒█░▒▓▒▒░   ▒▒   ▓▒█░░ ▒▓ ░▒▓░░░ ▒░ ░
      ░▒ ░ ▒░  ▒   ▒▒ ░▒ ░▒░    ▒   ▒▒ ░  ░▒ ░ ▒░ ░ ░  ░
      ░░   ░   ░   ▒   ░ ░ ░    ░   ▒     ░░   ░    ░   
       ░           ░  ░░   ░        ░  ░   ░        ░  ░
	`

// Media représente le résultat que le front‑end consomme.
type Media struct {
	Title    string `json:"title"`
	ID       int    `json:"id"`
	ThumbURL string `json:"thumbUrl"`
	Kind     string `json:"kind"` // "movie" | "series"
}

// Définition corrigée des structures pour correspondre au JSON réel
type PurestreamResponse struct {
	Data struct {
		Items struct {
			Movies struct {
				Items []PurestreamMovie `json:"items"`
			} `json:"movies"`
		} `json:"items"`
	} `json:"data"`
}

type PurestreamMovie struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Posters struct {
		Large string `json:"large"`
	} `json:"posters"`
}

var BaseURL string

func InitApp() {
	url, err := FetchBaseURL()
	if err != nil {
		log.Println("Impossible de détecter l'URL officielle, fallback :", err)
		BaseURL = "https://api.purstream.to"
		return
	}

	BaseURL = url
	log.Println("XalaFlix URL détectée :", BaseURL)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func FetchBaseURL() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://xalaflix.fr", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// PRIORITÉ : canonical
	if href, exists := doc.Find(`link[rel="canonical"]`).Attr("href"); exists {
		return strings.TrimRight(href, "/"), nil
	}

	// CTA principal
	if href, exists := doc.Find(".cta-btn").Attr("href"); exists {
		return strings.TrimRight(href, "/"), nil
	}

	// Carte principale
	if href, exists := doc.Find(".card a.button-primary").First().Attr("href"); exists {
		return strings.TrimRight(href, "/"), nil
	}

	return "", errors.New("base URL not found")
}

// fetchMedia parses the Xalaflix search page and returns a slice of Media.
func fetchMedia(ctx context.Context, query string) ([]Media, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	escaped := url.QueryEscape(query)
	// BaseURL doit être défini quelque part (ex: https://purstream.to)
	remote := fmt.Sprintf("%s/api/v1/search-bar/search/%s", BaseURL, escaped)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json") // On précise qu'on veut du JSON

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// On décode directement le flux JSON
	var apiData PurestreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiData); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}

	results := []Media{}

	// On boucle uniquement sur la section 'movies' du JSON
	for _, m := range apiData.Data.Items.Movies.Items {
		if m.Type == "movie" {
			results = append(results, Media{
				Title:    m.Title,
				ID:       m.ID,
				ThumbURL: m.Posters.Large,
				Kind:     "movie",
			})
		}
	}

	return results, nil
}

// sanitizeFileName enlève les caractères interdits sur Windows/macOS/Linux
func sanitizeFileName(name string) string {
	// Remplace les espaces multiples par un seul espace
	name = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(name, " "))

	// Supprime les caractères réservés : \ / : * ? " < > |
	illegal := regexp.MustCompile(`[\\/:*?"<>|]`)
	name = illegal.ReplaceAllString(name, "")

	// Limite la longueur (optionnel, 200 caractères max)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	media, err := fetchMedia(ctx, q)
	if err != nil {
		http.Error(w, "Erreur lors de la recherche : "+err.Error(),
			http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(media); err != nil {
		http.Error(w, "Erreur d'encodage JSON", http.StatusInternalServerError)
	}
}

type SheetResponse struct {
	Data struct {
		Items struct {
			ID   int `json:"id"`
			Urls []struct {
				URL  string `json:"url"`
				Name string `json:"name"`
			} `json:"urls"`
		} `json:"items"`
	} `json:"data"`
}

func getVideoURL(ctx context.Context, id int) (string, error) {
	// Construction de l'URL de la "sheet"
	remote := fmt.Sprintf("%s/api/v1/media/%d/sheet", BaseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Décodage du JSON
	var sheet SheetResponse
	if err := json.NewDecoder(resp.Body).Decode(&sheet); err != nil {
		return "", fmt.Errorf("failed to decode sheet JSON: %v", err)
	}

	// On vérifie si on a au moins une URL disponible dans le tableau 'urls'
	if len(sheet.Data.Items.Urls) > 0 {
		// On retourne la première URL (souvent la VF)
		return sheet.Data.Items.Urls[0].URL, nil
	}

	return "", fmt.Errorf("no video URL found for media %d", id)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	// ----------- Récupération des paramètres ----------
	detail := r.URL.Query().Get("detail")
	if detail == "" {
		http.Error(w, "missing ?detail= parameter", http.StatusBadRequest)
		return
	}

	rawTitle := r.URL.Query().Get("title")
	if rawTitle == "" {
		rawTitle = "video"
	}
	filename := sanitizeFileName(rawTitle) + ".mp4"

	// ----------- Obtenir l'URL du fichier vidéo ----------
	ctx := r.Context()

	detailID, err := strconv.Atoi(detail)
	if err != nil {
		http.Error(w, "invalid detail parameter: "+err.Error(),
			http.StatusBadRequest)
		return
	}

	videoURL, err := getVideoURL(ctx, detailID)
	if err != nil {
		http.Error(w, "cannot obtain video URL: "+err.Error(),
			http.StatusBadGateway)
		return
	}

	// ----------- Préparer la requête vers Xalaflix ----------
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		http.Error(w, "failed to build request: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
	req.Header.Set("Referer", BaseURL)
	req.Header.Set("Range", "bytes=0-")
	req.Header.Set("Accept-Encoding", "identity")

	client := &http.Client{Timeout: 0} // pas de timeout pour le streaming long
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "error fetching video: "+err.Error(),
			http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// ----------- Propagation des headers du fichier source ----------
	// (Content‑Type, Content‑Length, etc.)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "video/mp4")
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}

	// ----------- **FORCER LE TÉLÉCHARGEMENT** ----------
	// Cette ligne doit être **avant** tout WriteHeader.
	disposition := fmt.Sprintf(`attachment; filename="%s"`, filename)
	w.Header().Set("Content-Disposition", disposition)

	// Aucun WriteHeader explicite n'est nécessaire : le premier Write (io.Copy)
	// déclenchera automatiquement un 200 OK avec les headers déjà posés.

	// ----------- Copier le flux vers le client ----------
	_, copyErr := io.Copy(w, resp.Body)
	if copyErr != nil && !errors.Is(copyErr, net.ErrClosed) {
		fmt.Fprintf(os.Stderr, "stream copy error: %v\n", copyErr)
	}
}

func main() {
	fmt.Println(developerTag)

	InitApp()

	// Servir les fichiers UI
	http.Handle("/", http.FileServer(http.Dir("./ui")))

	// API recherche (déjà existante)
	http.HandleFunc("/api/search", searchHandler)

	// Nouvelle API téléchargement qui prend le paramètre `detail`
	http.HandleFunc("/api/download", downloadHandler)

	go func() {
		fmt.Println("XalaDownloader démarre sur http://127.0.0.1:8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}()

	time.Sleep(300 * time.Millisecond)

	// Ouvre le navigateur
	err := openBrowser("http://127.0.0.1:8080")
	if err != nil {
		log.Println("Impossible d'ouvrir le navigateur :", err)
	}

	// Empêche main de se terminer
	select {}
}
