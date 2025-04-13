package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/debugloop/wunschkonzert/pkg/realtime"
	spotifylib "github.com/debugloop/wunschkonzert/pkg/spotify"
	"github.com/debugloop/wunschkonzert/pkg/ui"
)

// SearchHandler returns the handler responsible for searching. It connects directly to the spotify search.
func SearchHandler(spotify *spotifylib.Client, market string, limit uint) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			err := req.ParseForm()
			if err != nil {
				slog.Error("Could not parse form.", "error", err)
				return
			}

			query := req.FormValue("search")
			if query == "" {
				return
			}

			slog.Info("Someone searched something.", "query", query)

			resp, err := spotify.Search(req.Context(), query, market, limit)
			if err != nil {
				slog.Error("Problem retrieving search results from spotify.", "error", err)
				return
			}

			err = ui.SearchResult(resp).Render(req.Context(), w)
			if err != nil {
				slog.Error("Unable to render or send response.", "error", err)
				return
			}
		},
	)
}

// NowPlaying is the handler returning the NowPlayingSection. It includes an initial render of the inner NowPlaying
// widget, which will be updated using SSE.
func NowPlayingHandler(spotify *spotifylib.Client) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			resp, err := spotify.NowPlaying(req.Context())
			if err != nil {
				slog.WarnContext(req.Context(), "Problem retrieving now playing data from spotify, rendering anyways.", "error", err)
			}

			err = ui.NowPlayingSection(resp).Render(req.Context(), w)
			if err != nil {
				slog.ErrorContext(req.Context(), "Unable to render or send response.", "error", err)
				return
			}
		},
	)
}

// NowPlayingLive is the SSE handler which continuously updates the widget inside NowPlayingSection.
func NowPlayingLiveHandler(realtimeService *realtime.Service, serverName string) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", serverName)
			w.Header().Set("Access-Control-Expose-Headers", "Content-Type")

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			updates := make(chan *spotifylib.NowPlaying)
			realtimeService.Subscribe(req.Context(), updates)
			defer realtimeService.Unsubscribe(req.Context(), updates)
			for {
				select {
				case <-req.Context().Done():
					return
				case np, ok := <-updates:
					if !ok {
						return
					}
					_, err := fmt.Fprintf(w, "data: ")
					if err != nil {
						slog.ErrorContext(req.Context(), "Unable to start SSE frame.", "error", err)
						return
					}
					err = ui.NowPlaying(np).Render(req.Context(), w)
					if err != nil {
						slog.ErrorContext(req.Context(), "Unable to render or send response.", "error", err)
						return
					}
					_, err = fmt.Fprintf(w, "\n\n")
					if err != nil {
						slog.ErrorContext(req.Context(), "Unable to complete SSE frame.", "error", err)
						return
					}
					w.(http.Flusher).Flush()
				}
			}
		},
	)
}

// AddHandler returns the handler accepting additions to a given playlist. It is passed directly to spotify and will
// return a disabled button if successful.
func AddHandler(spotify *spotifylib.Client, playlistID string) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			err := req.ParseForm()
			if err != nil {
				slog.Error("Could not parse form.", "error", err)
				return
			}

			song := req.FormValue("song")
			if song == "" {
				return
			}

			slog.Info("Someone has picked a song.", "song", song)

			err = spotify.AddToPlaylist(req.Context(), playlistID, song)
			if err != nil {
				slog.Error("Problem adding song to spotify playlist.", "error", err)
				return
			}

			err = ui.DisabledButton().Render(req.Context(), w)
			if err != nil {
				slog.Error("Unable to render or send response.", "error", err)
				return
			}
		},
	)
}
