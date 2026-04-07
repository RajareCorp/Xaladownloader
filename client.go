package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var BaseURL string

func InitApp() {
	startBaseURLRefresher(6 * time.Hour)
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
		return "https://api.purstream.art", fmt.Errorf("element .url-display introuvable")
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

func startBaseURLRefresher(interval time.Duration) {
	// Premier appel au démarrage pour initialiser la variable
	updateURL()

	// Ticker pour les prochaines mises à jour
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			updateURL()
		}
	}()
}

func updateURL() {
	newURL, err := FetchBaseURL()
	if err != nil {
		log.Printf("Erreur lors du rafraîchissement auto de l'URL : %v", err)
		return
	}

	// On met à jour la variable globale (BaseURL)
	// Idéalement, utilise un Mutex ici si tu as beaucoup de trafic
	BaseURL = newURL
	log.Printf("BaseURL mise à jour automatiquement : %s", BaseURL)
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
