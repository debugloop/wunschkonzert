package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"github.com/debugloop/wunschkonzert/pkg/auth"
	"github.com/debugloop/wunschkonzert/pkg/spotify"
)

var state = fmt.Sprintf("%d", rand.Int()) // this is more than good enough for a non-public auth endpoint

// OAuthLoginHandler returns a handler redirecting admin users to a spotify login page.
func OAuthLoginHandler(oauthService *auth.OAuthService) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			url := oauthService.Config().AuthCodeURL(state)
			http.Redirect(w, req, url, http.StatusTemporaryRedirect)
		},
	)
}

// OAuthCallbackHandler returns a handler that sets up our oauth info from admin users returning from the spotify login
// page.
func OAuthCallbackHandler(oauthService *auth.OAuthService, spotify *spotify.Client) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			gotState := req.URL.Query().Get("state")
			if state != gotState {
				http.Error(w, "Invalid state", http.StatusPreconditionFailed)
				return
			}

			code := req.URL.Query().Get("code")
			token, err := oauthService.Config().Exchange(req.Context(), code)
			if err != nil {
				http.Error(w, "Failed to exchange token", http.StatusUnauthorized)
				log.Println("Token exchange error:", err)
				return
			}

			oauthService.UseToken(token)

			http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		},
	)
}
