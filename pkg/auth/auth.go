package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/spotify"
)

// OAuthService implements everything needed to manage a spotify oauth process. It embeds the necessary config as well
// as some functionality to persist and restore tokens (and especially the refresh token) from disk.
type OAuthService struct {
	appCtx           context.Context
	config           *oauth2.Config
	token            *oauth2.Token
	tokenPersistPath string
}

// NewOAuthService returns a new OAuthService.
func NewOAuthService(appCtx context.Context, oauthRedirect string, clientID string, clientSecret string, tokenPersistPath string) *OAuthService {
	newService := &OAuthService{
		appCtx: appCtx,
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes: []string{
				"user-read-email",
				"user-read-recently-played",
				"user-read-currently-playing",
				"playlist-modify-public",
				"playlist-modify-private",
			},
			Endpoint:    spotify.Endpoint,
			RedirectURL: oauthRedirect + "/spotify/callback",
		},
		tokenPersistPath: tokenPersistPath,
	}

	if tokenPersistPath != "" {
		newService.restoreToken()
	}

	return newService
}

// Transport returns a self-authenticating http.RoundTripper from this service.
func (o *OAuthService) Transport() http.RoundTripper {
	return o.config.Client(o.appCtx, o.token).Transport
}

// Config returns this service's embedded config. This is needed to implement handlers.
func (o *OAuthService) Config() *oauth2.Config {
	return o.config
}

// UseToken receives a token which this service and it's Transports will use. It further persists the token to disk.
func (o *OAuthService) UseToken(token *oauth2.Token) {
	o.token = token

	jsonToken, err := json.Marshal(o.token)
	if err != nil {
		slog.Error("Could not persist token.", "error", err)
		return
	}

	if o.tokenPersistPath == "" {
		return
	}

	if err := os.WriteFile(o.tokenPersistPath, []byte(jsonToken), 0o600); err != nil {
		slog.Error("Writing of token file failed.", "error", err)
		return
	}
	slog.Info("Persisted token to disk.")
}

func (o *OAuthService) restoreToken() {
	tokenBytes, err := os.ReadFile(o.tokenPersistPath)
	if err != nil {
		slog.Warn("Reading of token file failed", "error", err)
		return
	}

	err = json.Unmarshal(tokenBytes, &o.token)
	if err != nil {
		slog.Error("Could not restore token.", "error", err)
		return
	}

	slog.Info("Restored token.")
}
