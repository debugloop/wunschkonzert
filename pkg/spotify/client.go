package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"

	"github.com/debugloop/wunschkonzert/pkg/auth"
)

// Client represents a spotify client. It implements only methods that are used for this app.
type Client struct {
	*http.Client
	oauthService *auth.OAuthService
	base         string
}

// New returns a new spotifyt client. The parameter should contain a self-authenticating RoundTripper.
func New(oauthService *auth.OAuthService) *Client {
	newClient := &Client{
		Client:       &http.Client{},
		oauthService: oauthService,
		base:         "https://api.spotify.com/v1",
	}
	newClient.refreshTransport()
	return newClient
}

func (c *Client) refreshTransport() {
	instrumentedTransport := otelhttp.NewTransport(
		c.oauthService.Transport(),
		otelhttp.WithMetricAttributesFn(
			func(r *http.Request) []attribute.KeyValue {
				return []attribute.KeyValue{
					attribute.String("path", r.URL.Path),
					attribute.String("method", r.Method),
				}
			},
		),
	)
	c.Transport = instrumentedTransport
}

func get[T any](c *Client, ctx context.Context, path string, params ...map[string]string) (*T, error) {
	pairedParams := []string{}
	for _, paramSet := range params {
		for k, v := range paramSet {
			pairedParams = append(pairedParams, fmt.Sprintf("%s=%s", k, v))
		}
	}
	appendableParams := strings.Join(pairedParams, "&")
	if appendableParams != "" {
		appendableParams = "?" + appendableParams
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.base+path+appendableParams, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		c.refreshTransport()
		return nil, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			slog.WarnContext(ctx, "Error closing Body after parsing the response.", "error", err)
		}
	}()

	switch resp.StatusCode {
	default:
		return nil, fmt.Errorf("received %s", resp.Status)
	case 204:
		return nil, nil
	case 200:
	}

	// Use this to integrate a new model:
	// raw, _ := io.ReadAll(resp.Body)
	// fmt.Println(string(raw))

	result := new(T)
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func post[T any](c *Client, ctx context.Context, path string, payload *T) error {
	reader := new(bytes.Buffer)
	err := json.NewEncoder(reader).Encode(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.base+path, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		c.refreshTransport()
		return err
	}

	switch code := resp.StatusCode; {
	default:
		return fmt.Errorf("received %s", resp.Status)
	case code >= 200 && code < 300:
		return nil
	}
}

// NowPlaying returns the currently playing song. You should rather use the realtime.Service to receive this
// information.
func (c *Client) NowPlaying(ctx context.Context) (*NowPlaying, error) {
	return get[NowPlaying](c, ctx, "/me/player/currently-playing")
}

// User returns information about the currently authenticated user.
func (c *Client) User(ctx context.Context) (*User, error) {
	return get[User](c, ctx, "/me")
}

// Search executes a search and returns the results.
func (c *Client) Search(ctx context.Context, query string, market string, limit uint) (*SearchResult, error) {
	return get[SearchResult](c, ctx, "/search", map[string]string{
		"q":      url.QueryEscape(query),
		"type":   "track",
		"market": market,
		"limit":  fmt.Sprintf("%d", limit),
	})
}

// AddToPlaylist adds a given song to a given playlist.
func (c *Client) AddToPlaylist(ctx context.Context, playlistID string, songUri string) error {
	req := &AddTracksToPlaylistReq{
		Uris:     []string{songUri},
		Position: 0,
	}
	return post(c, ctx, fmt.Sprintf("/playlists/%s/tracks", playlistID), req)
}
