package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"

	nanomesh "github.com/ccvass/swarmex/swarmex-nano-mesh"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	easytierBin := os.Getenv("EASYTIER_BIN")
	if easytierBin == "" {
		easytierBin = "/usr/local/bin/easytier-core"
	}
	peerEndpoint := os.Getenv("EASYTIER_PEER")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Error("failed to create Docker client", "error", err)
		os.Exit(1)
	}
	defer cli.Close()

	mesh := nanomesh.New(cli, easytierBin, peerEndpoint, logger)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "ok, %d peers", mesh.ActivePeers())
		})
		logger.Info("health endpoint", "addr", ":8080")
		http.ListenAndServe(":8080", nil)
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("swarmex-nano-mesh starting", "easytier", easytierBin, "peer", peerEndpoint)

	go mesh.OverlayMonitor(ctx, 30*time.Second)

	msgCh, errCh := cli.Events(ctx, events.ListOptions{})
	for {
		select {
		case event := <-msgCh:
			mesh.HandleEvent(ctx, event)
		case err := <-errCh:
			if ctx.Err() != nil {
				logger.Info("shutdown complete")
				return
			}
			logger.Error("event stream error", "error", err)
			return
		case <-ctx.Done():
			logger.Info("shutdown complete")
			return
		}
	}
}
