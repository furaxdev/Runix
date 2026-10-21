package daemon

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/furaxdev/runyx/internal/api"
)

// NewHTTPServer builds the daemon's HTTP server: the Ollama/OpenAI API
// routes behind the device-token auth middleware, bound to
// d.Config.ListenAddr.
func (d *Daemon) NewHTTPServer() *http.Server {
	mux := http.NewServeMux()
	apiServer := api.NewServer(d.Store, d.Manager, d.HW)
	apiServer.Register(mux)
	apiServer.RegisterRunyx(mux, &api.RunyxExtras{
		History:     d.History,
		Transcriber: d.Transcriber,
		Vision:      d.Vision,
	})
	registerStaticUI(mux)

	return &http.Server{
		Addr:              d.Config.ListenAddr,
		Handler:           d.Auth.Middleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

// Serve starts the HTTP server and the idle-eviction loop, blocking until
// ctx is cancelled, at which point it shuts everything down gracefully.
func (d *Daemon) Serve(ctx context.Context) error {
	d.StartEvictionLoop(ctx)
	defer d.Shutdown()

	srv := d.NewHTTPServer()
	ln, err := net.Listen("tcp", d.Config.ListenAddr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
