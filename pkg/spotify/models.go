package spotify

// AddTracksToPlaylistReq encodes a request to Spotify.
type AddTracksToPlaylistReq struct {
	Uris     []string `json:"uris"`
	Position uint     `json:"position"`
}

// SearchResult encodes a response from Spotify.
type SearchResult struct {
	Tracks SearchResultTracks `json:"tracks"`
}

// SearchResultTracks encodes a subset of a response from Spotify.
type SearchResultTracks struct {
	Songs []Song `json:"items"`
}

// NowPlaying encodes a response from Spotify.
type NowPlaying struct {
	LastChange uint   `json:"timestamp"`
	ProgressMs uint   `json:"progress_ms"`
	Type       string `json:"currently_playing_type"`
	Playing    bool   `json:"is_playing"`
	Song       Song   `json:"item"`
}

// Song encodes a subset of various responses from Spotify.
type Song struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	URI        string   `json:"uri"`
	Album      Album    `json:"album"`
	Artists    []Artist `json:"artists"`
	DurationMs uint     `json:"duration_ms"`
}

// Artist encodes a subset of various responses from Spotify.
type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	URI  string `json:"uri"`
}

// CoverImage encodes a subset of a response from Spotify.
type CoverImage struct {
	Height uint   `json:"height"`
	Width  uint   `json:"width"`
	URL    string `json:"url"`
}

// Album encodes a subset of a response from Spotify.
type Album struct {
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	Type                 string       `json:"album_type"`
	URI                  string       `json:"uri"`
	Artists              []Artist     `json:"artists"`
	CoverImages          []CoverImage `json:"images"`
	ReleaseDate          string       `json:"release_date"`
	ReleaseDatePrecision string       `json:"release_date_precision"`
}

// User encodes a subset of a response from Spotify.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"display_name"`
	Email string `json:"email"`
	Type  string `json:"type"`
	URI   string `json:"uri"`
}
