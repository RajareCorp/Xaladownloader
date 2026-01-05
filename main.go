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
	Title     string `json:"title"`
	DetailURL string `json:"detailUrl"`
	ThumbURL  string `json:"thumbUrl"`
	Kind      string `json:"kind"` // "movie" | "series"
}

type Season struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Thumb string `json:"thumbUrl"`
}

type Episode struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

var BaseURL string

func InitApp() {
	url, err := FetchBaseURL()
	if err != nil {
		log.Println("Impossible de détecter l'URL officielle, fallback :", err)
		BaseURL = "https://xalaflix.men"
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

	// Encode la requête pour éviter les caractères spéciaux.
	escaped := url.QueryEscape(query)
	remote := fmt.Sprintf(BaseURL+"/search_elastic?s=%s", escaped)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if err != nil {
		return nil, err
	}
	// Un User‑Agent raisonnable évite d'être bloqué par certains serveurs.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	results := []Media{}

	doc.Find(".single-video").Each(func(i int, s *goquery.Selection) {
		linkTag := s.Find("a")
		href, _ := linkTag.Attr("href")

		imgTag := s.Find("img")
		src, _ := imgTag.Attr("src")

		title := imgTag.AttrOr("alt", imgTag.AttrOr("title", ""))
		if title == "" {
			title = s.Find("span.video-item-content").Text()
		}

		if href == "" || src == "" || title == "" {
			return
		}

		kind := "movie"
		if strings.Contains(href, "/shows/details/") {
			kind = "series"
		}

		results = append(results, Media{
			Title:     title,
			DetailURL: href,
			ThumbURL:  src,
			Kind:      kind,
		})
	})

	return results, nil
}

func fetchSeasons(ctx context.Context, detailURL string) ([]Season, error) {
	base, _ := url.Parse(BaseURL)
	abs, _ := base.Parse(detailURL)

	req, _ := http.NewRequestWithContext(ctx, "GET", abs.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", BaseURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)

	seasons := []Season{}

	doc.Find(".season-item-related .single-video a").Each(func(i int, s *goquery.Selection) {
		title := s.AttrOr("title", "")
		href := s.AttrOr("href", "")
		img := s.Find("img").AttrOr("src", "")

		if title != "" && href != "" {
			seasons = append(seasons, Season{
				Title: title,
				URL:   href,
				Thumb: img,
			})
		}
	})

	return seasons, nil
}

func fetchEpisodes(ctx context.Context, seasonURL string) ([]Episode, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", seasonURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", BaseURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)

	episodes := []Episode{}

	doc.Find(".single-video a").Each(func(i int, s *goquery.Selection) {
		title := s.AttrOr("title", "")
		href := s.AttrOr("href", "")

		if title != "" && href != "" {
			episodes = append(episodes, Episode{
				Title: title,
				URL:   href,
			})
		}
	})

	return episodes, nil
}

func seasonsHandler(w http.ResponseWriter, r *http.Request) {
	detail := r.URL.Query().Get("detail")
	if detail == "" {
		http.Error(w, "missing detail", 400)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	seasons, err := fetchSeasons(ctx, detail)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}

	json.NewEncoder(w).Encode(seasons)
}

func episodesHandler(w http.ResponseWriter, r *http.Request) {
	season := r.URL.Query().Get("season")
	if season == "" {
		http.Error(w, "missing season", 400)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	episodes, err := fetchEpisodes(ctx, season)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}

	json.NewEncoder(w).Encode(episodes)
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

// getVideoURL scrapes the detail page and returns the src attribute
// found inside <div id="video-source"> (or any <source> tag there).
func getVideoURL(ctx context.Context, detailURL string) (string, error) {
	// On veut absolument un URL absolu. Si le lien est relatif,
	// on le résout par rapport à BaseURL
	base, err := url.Parse(BaseURL)
	if err != nil {
		return "", err
	}
	absDetail, err := base.Parse(detailURL)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, absDetail.String(), nil)
	if err != nil {
		return "", err
	}
	// Un UA standard évite d'être bloqué.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
	req.Header.Set("Referer", BaseURL)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("detail page returned %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	var videoURL string
	doc.Find("video#player source").Each(func(i int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok {
			videoURL = src
		}
	})

	if videoURL == "" {
		return "", fmt.Errorf("no video src found in %s", detailURL)
	}

	// Résolution d'un éventuel URL relatif
	u, err := url.Parse(videoURL)
	if err != nil {
		return "", err
	}
	if !u.IsAbs() {
		// On le rend absolu par rapport au domaine principal
		u = base.ResolveReference(u)
	}
	return u.String(), nil
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

	videoURL, err := getVideoURL(ctx, detail)
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

	http.HandleFunc("/api/series/seasons", seasonsHandler)
	http.HandleFunc("/api/series/episodes", episodesHandler)

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
