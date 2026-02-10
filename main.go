package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

type Media struct {
	Title    string `json:"title"`
	ID       int    `json:"id"`
	ThumbURL string `json:"thumbUrl"`
	Kind     string `json:"kind"`
	Runtime  string `json:"runtime"`
	Updated  string `json:"updatedAt"`
}

type Episode struct {
	Number int    `json:"episode"`
	Name   string `json:"name"`
}

// --- Structures API ---

type PurestreamMovie struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Type            string `json:"type"`              // Movie ou TV
	Runtime         int    `json:"runtime"`           // Changé : int au lieu de string
	UpdatedAt       string `json:"release_date"`      // Changé : mapping sur release_date
	LargePosterPath string `json:"large_poster_path"` // Nouveau : remplace Posters.Large
}

type PurestreamResponse struct {
	Data struct {
		Items struct {
			Movies struct {
				Items []PurestreamMovie `json:"items"`
			} `json:"movies"`
		} `json:"items"`
	} `json:"data"`
}

// Structure pour mapper le JSON brut de l'API /last-released-movies/
type LastReleasesAPIResponse struct {
	Data struct {
		Items []struct {
			ID        int    `json:"id"`
			Title     string `json:"title"`
			Type      string `json:"type"`
			Runtime   string `json:"runtime"`
			UpdatedAt string `json:"updatedAt"`
			Posters   struct {
				Large string `json:"large"`
			} `json:"posters"`
		} `json:"items"`
	} `json:"data"`
}

type FranchiseAPIResponse struct {
	Data struct {
		Items struct {
			Franchise struct {
				Movies struct {
					Items []struct {
						ID              int    `json:"id"`
						Title           string `json:"title"`
						Type            string `json:"type"`
						Runtime         int    `json:"runtime"`           // Changé en int
						LargePosterPath string `json:"large_poster_path"` // Nouveau nom
						UpdatedAt       string `json:"release_date"`      // On utilise release_date comme fallback
					} `json:"items"`
				} `json:"movies"`
			} `json:"franchise"`
		} `json:"items"`
	} `json:"data"`
}

type SheetResponse struct {
	Data struct {
		Items struct {
			ID      int    `json:"id"`
			Type    string `json:"type"`
			Seasons int    `json:"seasons"`
			Urls    []struct {
				URL  string `json:"url"`
				Name string `json:"name"`
			} `json:"urls"`
		} `json:"items"`
	} `json:"data"`
}

type StreamResponse struct {
	Data struct {
		Items struct {
			Sources []struct {
				StreamURL  string `json:"stream_url"`
				SourceName string `json:"source_name"`
			} `json:"sources"`
		} `json:"items"`
	} `json:"data"`
}

type SeasonDetailResponse struct {
	Data struct {
		Items struct {
			Episodes []Episode `json:"episodes"`
		} `json:"items"`
	} `json:"data"`
}

var BaseURL string

// --- Logique App ---

func InitApp() {
	url, err := FetchBaseURL()
	if err != nil {
		BaseURL = "https://api.purstream.to"
		return
	}
	BaseURL = url
	log.Println("URL détectée :", BaseURL)
}

func FetchBaseURL() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get("https://purstream.wiki")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// 1. Extraction de l'URL brute depuis la classe url-display
	rawURL, exists := doc.Find("a.url-display").First().Attr("href")
	if !exists || rawURL == "" {
		return "https://api.purstream.to", fmt.Errorf("element .url-display introuvable")
	}

	// 2. Parsing de l'URL pour manipuler le Host
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("URL invalide : %v", err)
	}

	// 3. Transformation du Host : purstream.me -> api.purstream.me
	// On nettoie d'abord d'éventuels préfixes existants (au cas où)
	host := strings.TrimPrefix(u.Host, "www.")
	host = strings.TrimPrefix(host, "api.") // Sécurité si l'URL est déjà api.

	u.Host = "api." + host
	u.Path = "" // On s'assure que le chemin est vide pour avoir juste la base
	u.Scheme = "https"

	return strings.TrimRight(u.String(), "/"), nil
}

func fetchMedia(ctx context.Context, query string) ([]Media, error) {
	remote := fmt.Sprintf("%s/api/v1/search-bar/search/%s", BaseURL, url.QueryEscape(query))
	req, _ := http.NewRequestWithContext(ctx, "GET", remote, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiData PurestreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiData); err != nil {
		log.Printf("Erreur décodage Search: %v", err)
		return nil, err
	}

	results := []Media{}
	for _, m := range apiData.Data.Items.Movies.Items {
		results = append(results, Media{
			Title:    m.Title,
			ID:       m.ID,
			ThumbURL: m.LargePosterPath, // On utilise le nouveau champ direct
			Kind:     m.Type,
			Runtime:  fmt.Sprintf("%d min", m.Runtime), // Conversion int -> string
			Updated:  m.UpdatedAt,
		})
	}
	return results, nil
}

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

// --- Handlers ---

func lastReleasesHandler(w http.ResponseWriter, r *http.Request) {
	remote := fmt.Sprintf("%s/api/v1/last-released-movies/13", BaseURL)
	req, _ := http.NewRequestWithContext(r.Context(), "GET", remote, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Erreur API externe", 502)
		return
	}
	defer resp.Body.Close()

	// L'API renvoie désormais un tableau d'items directement dans Data
	var apiData struct {
		Data struct {
			Items []PurestreamMovie `json:"items"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiData); err != nil {
		log.Printf("Erreur décodage LastReleases: %v", err)
		http.Error(w, "Erreur décodage JSON", 500)
		return
	}

	finalResults := []Media{}
	for _, item := range apiData.Data.Items {
		finalResults = append(finalResults, Media{
			Title:    item.Title,
			ID:       item.ID,
			ThumbURL: item.LargePosterPath, // Correction ici
			Kind:     item.Type,
			Runtime:  fmt.Sprintf("%d min", item.Runtime), // Conversion int -> string
			Updated:  item.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResults)
}

func franchiseHandler(w http.ResponseWriter, r *http.Request) {
	franchiseID := r.URL.Query().Get("id")
	if franchiseID == "" {
		franchiseID = "30" // Par défaut Prime Video
	}

	remote := fmt.Sprintf("%s/api/v1/franchise/%s", BaseURL, franchiseID)
	req, _ := http.NewRequestWithContext(r.Context(), "GET", remote, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Erreur API Franchise", 502)
		return
	}
	defer resp.Body.Close()

	var apiData FranchiseAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiData); err != nil {
		log.Printf("Erreur décodage précise : %v", err)
		http.Error(w, "Erreur décodage Franchise", 500)
		return
	}

	finalResults := []Media{}
	for _, item := range apiData.Data.Items.Franchise.Movies.Items {
		finalResults = append(finalResults, Media{
			Title:    item.Title,
			ID:       item.ID,
			ThumbURL: item.LargePosterPath, // Utilise le nouveau champ
			Kind:     item.Type,
			// Conversion de l'int runtime en string pour rester compatible avec ton type Media
			Runtime: fmt.Sprintf("%d min", item.Runtime),
			Updated: item.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResults)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	res, _ := fetchMedia(r.Context(), q)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func episodesHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	season := r.URL.Query().Get("num")

	// On force le formatage "01" pour l'API si nécessaire
	remote := fmt.Sprintf("%s/api/v1/media/%s/season/%s", BaseURL, id, season)

	req, _ := http.NewRequest("GET", remote, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	// Paramètres communs
	detailID := r.URL.Query().Get("detail")
	infoOnly := r.URL.Query().Get("infoOnly") == "true"

	// Paramètres spécifiques aux séries
	season := r.URL.Query().Get("season")
	episode := r.URL.Query().Get("episode")

	if detailID == "" {
		http.Error(w, "ID manquant", 400)
		return
	}

	// --- CAS A : Récupération des infos (Sheet) ---
	if infoOnly {
		remote := fmt.Sprintf("%s/api/v1/media/%s/sheet", BaseURL, detailID)
		resp, err := http.Get(remote)
		if err != nil {
			log.Printf("Erreur appel API Sheet: %v", err)
			http.Error(w, "Erreur API Sheet", 502)
			return
		}
		defer resp.Body.Close()

		var sheet SheetResponse
		// On lit tout le corps pour pouvoir le logger en cas d'erreur
		body, _ := io.ReadAll(resp.Body)

		if err := json.Unmarshal(body, &sheet); err != nil {
			log.Printf("Erreur décodage JSON: %v | Body: %s", err, string(body))
			http.Error(w, "Erreur décodage Sheet", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Construction de la réponse pour le JS
		// Note: on renvoie un tableau vide pour "urls" pour éviter l'erreur .length sur le JS
		out := map[string]interface{}{
			"data": map[string]interface{}{
				"items": map[string]interface{}{
					"id":           sheet.Data.Items.ID,
					"season_count": sheet.Data.Items.Seasons,
					"type":         sheet.Data.Items.Type,
					"urls":         sheet.Data.Items.Urls,
				},
			},
		}

		// Si Seasons est à 0 mais que c'est une TV, on force à 1 pour l'affichage
		if sheet.Data.Items.Type == "tv" && sheet.Data.Items.Seasons == 0 {
			out["data"].(map[string]interface{})["items"].(map[string]interface{})["season_count"] = 1
		}

		json.NewEncoder(w).Encode(out)
		return
	}

	// --- CAS B : Téléchargement (Film ou Série) ---
	var streamRemote string
	if season != "" && episode != "" {
		// C'est une série
		streamRemote = fmt.Sprintf("%s/api/v1/stream/%s/episode?season=%s&episode=%s", BaseURL, detailID, season, episode)
	} else {
		// C'est un film
		streamRemote = fmt.Sprintf("%s/api/v1/stream/%s", BaseURL, detailID)
	}

	sResp, err := http.Get(streamRemote)
	if err != nil || sResp.StatusCode != 200 {
		http.Error(w, "Erreur récupération lien stream", 502)
		return
	}
	defer sResp.Body.Close()

	var streamData StreamResponse
	if err := json.NewDecoder(sResp.Body).Decode(&streamData); err != nil || len(streamData.Data.Items.Sources) == 0 {
		http.Error(w, "Source introuvable", 404)
		return
	}

	// Transformation de l'URL pour le téléchargement direct
	rawURL := streamData.Data.Items.Sources[0].StreamURL
	finalDownloadURL := strings.Replace(rawURL, "/stream?", "/?", 1)

	// Préparation du nom de fichier
	title := r.URL.Query().Get("title")
	if season != "" && episode != "" {
		title = fmt.Sprintf("%s S%sE%s", title, season, episode)
	}
	filename := sanitizeFileName(title) + ".mp4"

	// Proxy du téléchargement
	req, _ := http.NewRequest("GET", finalDownloadURL, nil)
	req.Header.Set("Referer", BaseURL)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 0}
	res, err := client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "video/mp4")
	io.Copy(w, res.Body)
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
	InitApp()

	http.Handle("/", http.FileServer(http.Dir("./ui")))
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
