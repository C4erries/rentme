package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

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

	session, err := scylla.NewSession(cfg, logger)
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
