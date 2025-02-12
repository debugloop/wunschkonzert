package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/a-h/templ"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/debugloop/wunschkonzert/pkg/api"
	"github.com/debugloop/wunschkonzert/pkg/api/handlers"
	"github.com/debugloop/wunschkonzert/pkg/auth"
	"github.com/debugloop/wunschkonzert/pkg/realtime"
	spotifylib "github.com/debugloop/wunschkonzert/pkg/spotify"
	"github.com/debugloop/wunschkonzert/pkg/ui"
)

func main() {
	// App server settings.
	serverName := flag.String("server.name", "http://localhost:8080", "The public address of the server. Used for CORS and the oauth redirect.")
	serverListen := flag.String("server.listen", ":8080", "Where the app will be listening for the user-facing routes.")

	// Spotify Authentication.
	authListen := flag.String("auth.listen", ":8081", "Where the app will be listening for the admin's spotify login.")
	clientID := flag.String("auth.client.id", "", "The OAuth Client ID")
	clientSecret := flag.String("auth.client.secret", "", "The OAuth Client Secret")
	tokenPersistPath := flag.String("auth.token.path", "./token.json", "The path where a token will be persisted. May be empty in order to not persist tokens.")

	// Spotify settings.
	nowPlayingFrequency := flag.Duration("nowplaying.frequency", 1*time.Second, "The frequency of now playing info updates")
	searchMarket := flag.String("search.market", "DE", "The market that searching is limited to")
	searchLimit := flag.Uint("search.limit", 15, "The number of results that searching is limited to")
	playlistID := flag.String("playlist.id", "", "The ID of the playlist users can prepend to")

	// Observability.
	metricsListen := flag.String("metrics.listen", ":9999", "Where the app will be exposing its metrics.")
	verbose := flag.Bool("verbose", false, "Whether to be more verbose in logging")

	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(
		os.Stdout,
		&slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		},
	)))

	if *clientID == "" || *clientSecret == "" || *playlistID == "" {
		if *clientID == "" {
			slog.Error("Missing required -auth.client.id argument.")
		}
		if *clientSecret == "" {
			slog.Error("Missing required -auth.client.secret argument.")
		}
		if *playlistID == "" {
			slog.Error("Missing required -playlist.id argument.")
		}
		os.Exit(2)
	}

	otelSink, err := prometheus.New()
	if err != nil {
		slog.Error("Failed to setup opentelemetry prometheus collector", "error", err)
	}
	provider := metric.NewMeterProvider(metric.WithReader(otelSink))
	otel.SetMeterProvider(provider)

	metricServer := api.NewServer("metrics", *metricsListen)
	metricServer.Handle("/metrics", promhttp.Handler())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Setup our OAuth service, which will restore and persist a token it has obtained. It will obtain those through
	// the admin api handlers which have access to this service.
	oauthService := auth.NewOAuthService(ctx, *serverName, *clientID, *clientSecret, *tokenPersistPath)

	// Setup spotify adapter. As it is an authenticated API, it uses the client provided by the oauthService, as
	// that will use a token automatically. The context is used for refreshing the token.
	spotify := spotifylib.New(oauthService)

	// Setup our realtime service, which gets the now playing song from spotify at an interval and multiplexes the
	// info to all users.
	spotifyRealtimeSubscription := realtime.NewService(spotify, *nowPlayingFrequency)
	spotifyRealtimeSubscription.Start(ctx)

	// Make an initial request to spotify to log some info about our credentials.
	user, err := spotify.User(ctx)
	if err != nil {
		slog.Error("Token is invalid.", "error", err)
	} else {
		slog.Info("Token is valid.", "username", user.ID)
	}

	// Expose regular handlers on one listener.
	userServer := api.NewServer("user", *serverListen)
	userServer.Handle("/", templ.Handler(ui.Index()))
	userServer.Handle("/now-playing", handlers.NowPlayingHandler(
		spotify, // Used for the initial page render only.
	))
	userServer.Handle("/now-playing-live", handlers.NowPlayingLiveHandler(
		spotifyRealtimeSubscription, // Used to subscribe to continuous live updates.
		*serverName,                 // Used for CORS headers.
	))
	userServer.Handle("POST /search", handlers.SearchHandler(
		spotify,       // Used to facilitate search.
		*searchMarket, // Limit to the given market area.
		*searchLimit,  // Limit to a number of results.
	))
	userServer.Handle("POST /add", handlers.AddHandler(
		spotify,     // Used to facilitate adding to playlists.
		*playlistID, // What playlist to add to.
	))

	// Expose admin handlers on different listeners, admin listener for initiation and public for callback.
	adminServer := api.NewServer("admin", *authListen)
	adminServer.Handle("/", handlers.OAuthLoginHandler(
		oauthService,
	))
	userServer.Handle("/spotify/callback", handlers.OAuthCallbackHandler(
		oauthService,
		spotify,
	))

	// Orchestrate all servers to run and shutdown later.
	eg := api.Orchestrate(ctx, 5*time.Second, userServer, adminServer, metricServer)
	if err = eg.Wait(); err != nil {
		slog.Error("Unclean termination of at least one server.", "error", err)
	}

	slog.Info("Exiting, kthxbye.")
}
