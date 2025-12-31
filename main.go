package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
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

type Config struct {
	BaseURL string `json:"base_url"`
}

// global (thread‑safe) configuration
var (
	cfgMu sync.RWMutex
	cfg   Config
)

// loadConfig lit le fichier (ou crée un défaut) au démarrage
func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		// Si le fichier n'existe pas, on crée une config par défaut
		if errors.Is(err, os.ErrNotExist) {
			cfg = Config{BaseURL: cfg.BaseURL}
			return saveConfig(path) // crée le fichier avec la valeur par défaut
		}
		return err
	}
	return json.Unmarshal(data, &cfg)
}

// saveConfig écrit la configuration courante sur disque
func saveConfig(path string) error {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
func adminSetBaseURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	type payload struct {
		BaseURL string `json:"base_url"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if p.BaseURL == "" {
		http.Error(w, "`base_url` cannot be empty", http.StatusBadRequest)
		return
	}

	// Mise à jour atomique
	cfgMu.Lock()
	cfg.BaseURL = strings.TrimSpace(p.BaseURL)
	cfgMu.Unlock()

	// Persistance sur disque
	if err := saveConfig("config.json"); err != nil {
		http.Error(w, "failed to persist config: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204 : tout s'est bien passé
}

// Media représente le résultat que le front‑end consomme.
type Media struct {
	Title     string `json:"title"`
	DetailURL string `json:"detailUrl"`
	ThumbURL  string `json:"thumbUrl"`
}

// fetchMedia parses the Xalaflix search page and returns a slice of Media.
func fetchMedia(ctx context.Context, query string) ([]Media, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// Encode la requête pour éviter les caractères spéciaux.
	escaped := url.QueryEscape(query)
	remote := fmt.Sprintf(cfg.BaseURL+"/search_elastic?s=%s", escaped)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if err != nil {
		return nil, err
	}
	// Un User‑Agent raisonnable évite d'être bloqué par certains serveurs.
	req.Header.Set("User-Agent", "LumoBot/1.0 (+https://proton.me)")

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

	// -----------------------------------------------------------------
	// Films (bloc "Films”)
	// -----------------------------------------------------------------
	doc.Find(".section .vfx-item-section + .row .single-video").Each(func(i int, s *goquery.Selection) {
		linkTag := s.Find("a")
		href, _ := linkTag.Attr("href")
		imgTag := s.Find("img")
		src, _ := imgTag.Attr("src")
		title := imgTag.AttrOr("alt", imgTag.AttrOr("title", ""))
		if title == "" {
			// parfois le titre est dans le span.video-item-content
			title = s.Find("span.video-item-content").Text()
		}
		if href != "" && src != "" && title != "" {
			results = append(results, Media{
				Title:     title,
				DetailURL: href,
				ThumbURL:  src,
			})
		}
	})

	// -----------------------------------------------------------------
	// Séries (bloc "Séries”) – même structure, donc on ré‑utilise le sélecteur
	// -----------------------------------------------------------------
	doc.Find(".section.section-padding.bg-image.tv_show .single-video").Each(func(i int, s *goquery.Selection) {
		linkTag := s.Find("a")
		href, _ := linkTag.Attr("href")
		imgTag := s.Find("img")
		src, _ := imgTag.Attr("src")
		title := imgTag.AttrOr("alt", imgTag.AttrOr("title", ""))
		if title == "" {
			title = s.Find("span.video-item-content").Text()
		}
		if href != "" && src != "" && title != "" {
			results = append(results, Media{
				Title:     title,
				DetailURL: href,
				ThumbURL:  src,
			})
		}
	})

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

// getVideoURL scrapes the detail page and returns the src attribute
// found inside <div id="video-source"> (or any <source> tag there).
func getVideoURL(ctx context.Context, detailURL string) (string, error) {
	// On veut absolument un URL absolu. Si le lien est relatif,
	// on le résout par rapport à cfg.BaseURL
	base, err := url.Parse(cfg.BaseURL)
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
	req.Header.Set("Referer", cfg.BaseURL)

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
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

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
	req.Header.Set("Referer", cfg.BaseURL)
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
	if err := loadConfig("config.json"); err != nil {
		fmt.Fprintf(os.Stderr, "Impossible de charger la config : %v\n", err)
		os.Exit(1)
	}

	// Servir les fichiers UI
	http.Handle("/", http.FileServer(http.Dir("./ui")))

	http.HandleFunc("/admin/base-url", adminSetBaseURLHandler)

	// API recherche (déjà existante)
	http.HandleFunc("/api/search", searchHandler)

	// Nouvelle API téléchargement qui prend le paramètre `detail`
	http.HandleFunc("/api/download", downloadHandler)

	fmt.Println("🚀 XalaDownloader démarre sur :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
