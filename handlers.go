package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	res, _ := fetchMedia(r.Context(), q)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

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
