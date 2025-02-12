package realtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	spotifylib "github.com/debugloop/wunschkonzert/pkg/spotify"
)

// Service is a long running service which regularly queries spotify and provides realtime data to all subscribers. This
// means all subscribers can share a single realtime data source instead of querying on their own.
type Service struct {
	sync.RWMutex
	o           sync.Once
	t           time.Ticker
	subscribers map[chan *spotifylib.NowPlaying]struct{}

	spotify *spotifylib.Client
}

// NewService returns a new Service ready for use.
func NewService(spotify *spotifylib.Client, frequency time.Duration) *Service {
	return &Service{
		o:           sync.Once{},
		t:           *time.NewTicker(frequency),
		subscribers: make(map[chan *spotifylib.NowPlaying]struct{}),
		spotify:     spotify,
	}
}

// Start spawns a go routine to run the realtime source.
func (s *Service) Start(ctx context.Context) {
	go s.o.Do(func() {
		s.run(ctx)
	})
}

// Subscribe accepts channels which will be fed from the realtime source.
func (s *Service) Subscribe(sub chan *spotifylib.NowPlaying) {
	s.Lock()
	defer s.Unlock()
	s.subscribers[sub] = struct{}{}
	slog.Debug("Someone just opened the page.", "current-user-count", len(s.subscribers))
}

// Unsubscribe ends a subscription. This will close the channel.
func (s *Service) Unsubscribe(sub chan *spotifylib.NowPlaying) {
	s.Lock()
	defer s.Unlock()
	delete(s.subscribers, sub)
	close(sub)
	slog.Debug("Someone just closed the page.", "current-user-count", len(s.subscribers))
}

func (s *Service) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.t.C:
			np, err := s.spotify.NowPlaying(ctx)
			if err != nil {
				slog.Error("Could not retrieve now-playing data.", "error", err)
				continue
			}
			if np == nil {
				continue
			}
			s.publish(np)
		}
	}
}

func (s *Service) publish(np *spotifylib.NowPlaying) {
	s.Lock()
	defer s.Unlock()
	for sub := range s.subscribers {
		select {
		case sub <- np:
		case <-time.After(200 * time.Millisecond):
			slog.Warn("Closing unresponsive user stream.", "current-user-count", len(s.subscribers))
			delete(s.subscribers, sub)
			close(sub)
		}
	}
}
