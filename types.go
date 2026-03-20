package main

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

type SheetURL struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type SheetResponse struct {
	Data struct {
		Items struct {
			ID      int        `json:"id"`
			Type    string     `json:"type"`
			Title   string     `json:"title"`
			Urls    []SheetURL `json:"urls"`
			Seasons int        `json:"seasons"`
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
