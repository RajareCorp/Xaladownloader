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

/*
Handler pour récupérer les dernières sorties (films/séries récemment ajoutés).
L'UI peut afficher ces éléments dans une section "Dernières sorties" ou similaire.
*/
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

/*
Handler pour récupérer les franchises (collections) disponibles.
L'UI peut ensuite afficher ces franchises pour que l'utilisateur puisse les sélectionner.
*/
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

/*
Handler pour récupérer les épisodes d'une saison spécifique.
L'UI envoie l'ID du média et le numéro de saison, et ce handler interroge l'API pour obtenir la liste des épisodes.
*/
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

/*
Handler pour le téléchargement : Ce handler reçoit l'ID du média et la saison (si applicable), récupère la feuille de route (sheet) correspondante,
et gère à la fois le mode "infoOnly" (renvoi de la liste des URLs à l'UI) et le mode "download" (lancement du téléchargement ou redirection).
*/
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	detailID := r.URL.Query().Get("detail")
	infoOnly := r.URL.Query().Get("infoOnly") == "true"
	selectedURL := r.URL.Query().Get("selectedUrl") // Nouveau paramètre

	if detailID == "" {
		http.Error(w, "ID manquant", 400)
		return
	}

	// --- ÉTAPE 1 : Récupération de la Sheet ---
	remote := fmt.Sprintf("%s/api/v1/media/%s/sheet", BaseURL, detailID)
	resp, err := http.Get(remote)
	if err != nil {
		http.Error(w, "Erreur API Sheet", 502)
		return
	}
	defer resp.Body.Close()

	var sheet SheetResponse
	json.NewDecoder(resp.Body).Decode(&sheet)

	// --- ÉTAPE 2 : Mode Info (Renvoi de la liste à l'UI) ---
	if infoOnly && selectedURL == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sheet)
		return
	}

	// --- ÉTAPE 3 : Traitement du téléchargement ---
	targetURL := selectedURL
	// Si l'utilisateur n'a pas encore choisi mais veut télécharger, on prend la 1ère par défaut
	if targetURL == "" && len(sheet.Data.Items.Urls) > 0 {
		targetURL = sheet.Data.Items.Urls[0].URL
	}

	if targetURL == "" {
		http.Error(w, "Aucune URL valide trouvée", 404)
		return
	}

	// --- ÉTAPE 4 : Validation et Adaptation (MP4 vs M3U8) ---
	if strings.Contains(targetURL, ".m3u8") {
		// Logique pour M3U8 : Ici, on peut soit rediriger,
		// soit utiliser un outil comme ffmpeg en arrière plan.
		// Pour un proxy simple, on va juste rediriger ou prévenir.
		http.Redirect(w, r, targetURL, http.StatusTemporaryRedirect)
		return
	}

	// --- ÉTAPE 5 : Proxy de téléchargement pour MP4 ---
	downloadFileProxy(w, targetURL, sheet.Data.Items.Title)
}

/*
Proxy de téléchargement : Ce handler agit comme un intermédiaire pour télécharger le fichier depuis l'URL source et le servir directement à l'utilisateur,
tout en gérant les headers pour la progression et le nom de fichier.
*/
func downloadFileProxy(w http.ResponseWriter, targetURL string, title string) {
	// 1. On récupère le fichier source
	res, err := http.Get(targetURL)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du fichier", 502)
		return
	}
	defer res.Body.Close()

	// 2. IMPORTANT : On transfère la taille du fichier pour la barre de progression
	if contentLength := res.Header.Get("Content-Length"); contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}

	// 3. Autoriser le JS à lire les headers (pour XMLHttpRequest)
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Disposition")

	filename := sanitizeFileName(title) + ".mp4"
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "video/mp4")

	// 4. On stream le contenu
	io.Copy(w, res.Body)
}

/*
Handler pour lancer le téléchargement d'un flux M3U8.
L'UI envoie l'URL du flux et le titre pour le nom de fichier.
*/
func m3u8Handler(w http.ResponseWriter, r *http.Request) {
	streamURL := r.URL.Query().Get("url")
	title := r.URL.Query().Get("title")

	if streamURL == "" || title == "" {
		http.Error(w, "Paramètres manquants", 400)
		return
	}

	// On lance le téléchargement dans une Goroutine pour ne pas bloquer le navigateur
	go func() {
		err := DownloadM3U8(streamURL, title)
		if err != nil {
			fmt.Println("Erreur M3U8:", err)
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Téléchargement lancé"))
}

/*
Vérification de l'URL pour éliminer les liens morts avant de lancer le téléchargement/streaming.
*/
func checkURLHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	client := &http.Client{Timeout: 3 * time.Second}

	// On utilise HEAD pour ne pas consommer de bande passante
	resp, err := client.Head(target)

	status := "ok"
	if err != nil || resp.StatusCode >= 400 {
		status = "dead"
	}

	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

var m3u8Progress = make(map[string]string)

/*
Handler pour vérifier le statut du téléchargement M3U8 en cours.
L'UI peut interroger cette route pour afficher une progression ou un message d'état.
*/
func m3u8StatusHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	status, ok := m3u8Progress[title]
	if !ok {
		status = "Aucun téléchargement en cours"
	}

	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

/*
Handler pour récupérer le catalogue par type (movie, tv, anime).
Exemple : /api/catalog?type=anime
*/
func catalogHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Récupération et validation du type et de la page demandés
	contentType := r.URL.Query().Get("type")
	if contentType == "" {
		contentType = "movie" // Défaut
	}

	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	// 2. Construction de l'URL avec les paramètres demandés
	// Note: On utilise la variable globale BaseURL mise à jour par ton refresher
	remote := fmt.Sprintf("%s/api/v1/catalog/movies?sortBy=best-rated&types=%s&perPage=100&page=%s", BaseURL, contentType, page)

	req, _ := http.NewRequestWithContext(r.Context(), "GET", remote, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Erreur API Catalog: %v", err)
		http.Error(w, "Erreur lors de l'appel à l'API source", 502)
		return
	}
	defer resp.Body.Close()

	// 3. Structure correspondant exactement au JSON fourni
	var apiResponse struct {
		Data struct {
			Items struct {
				Data []PurestreamMovie `json:"data"` // Le tableau est ici
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		log.Printf("Erreur décodage Catalog: %v", err)
		http.Error(w, "Erreur format de données", 500)
		return
	}

	// 4. Transformation vers ton type interne 'Media' (comme dans tes autres handlers)
	finalResults := []Media{}
	for _, item := range apiResponse.Data.Items.Data {
		finalResults = append(finalResults, Media{
			Title:    item.Title,
			ID:       item.ID,
			ThumbURL: item.LargePosterPath,
			Kind:     item.Type,
			Runtime:  fmt.Sprintf("%d min", item.Runtime),
			Updated:  item.UpdatedAt,
		})
	}

	// 5. Envoi de la réponse à l'UI
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResults)
}
