package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gocql/gocql"
	"google.golang.org/grpc"

	"messaging-service/internal/config"
	"messaging-service/internal/obs"
	"messaging-service/internal/service"
	"messaging-service/internal/storage/scylla"
	pb "messaging-service/proto"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		logger := obs.NewLogger("dev")
		logger.Error("config load failed", "error", err)
		os.Exit(1)
	}
	logger := obs.NewLogger(cfg.Env)

	session, err := connectScyllaWithRetry(ctx, cfg, logger)
	if err != nil {
		logger.Error("scylla init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		session.Close()
	}()

	grpcServer := grpc.NewServer()
	store := scylla.NewStore(session, logger)
	pb.RegisterMessagingServiceServer(grpcServer, &service.Server{
		Store:  store,
		Logger: logger,
	})

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		logger.Error("failed to listen", "error", err, "addr", cfg.GRPCAddr)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down grpc server")
		grpcServer.GracefulStop()
	}()

	logger.Info("messaging-service starting", "addr", cfg.GRPCAddr, "env", cfg.Env)
	if err := grpcServer.Serve(lis); err != nil {
		if err == grpc.ErrServerStopped {
			logger.Info("grpc server stopped")
			return
		}
		logger.Error("grpc server failed", "error", err)
		os.Exit(1)
	}
	logger.Info("messaging-service stopped")
}

func connectScyllaWithRetry(ctx context.Context, cfg config.Config, logger *slog.Logger) (*gocql.Session, error) {
	const maxWait = 2 * time.Minute

	deadline := time.Now().Add(maxWait)
	backoff := 500 * time.Millisecond
	const maxBackoff = 10 * time.Second

	var lastErr error
	for attempt := 1; ; attempt++ {
		session, err := scylla.NewSession(cfg, logger)
		if err == nil {
			return session, nil
		}
		lastErr = err

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		if logger != nil {
			logger.Warn("scylla not ready, retrying", "attempt", attempt, "error", err, "backoff", backoff.String())
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
