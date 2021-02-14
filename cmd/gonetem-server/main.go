package main

import (
	"context"
	"flag"
	stdlog "log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log/level"
	"github.com/mroy31/gonetem/internal/logger"
	"github.com/mroy31/gonetem/internal/options"
	pb "github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/server"
	"google.golang.org/grpc"
)

var (
	grpcServer *grpc.Server = nil
	socket     net.Listener = nil
	verbose                 = flag.Bool("verbose", false, "Display more messages")
	conf                    = flag.String("conf-file", "", "Configuration path")
	logFile                 = flag.String("log-file", "", "Path of the log file")
)

func main() {
	flag.Parse()
	options.InitServerConfig()

	// init logger
	logWriter := os.Stderr
	if *logFile != "" {
		f, err := os.Create(*logFile)
		if err != nil {
			stdlog.Fatalf("Unable to create log file %s: %v", *logFile, err)
		}
		defer f.Close()

		logWriter = f
	}
	logger.InitLogger(logWriter, level.AllowError(), level.AllowWarn(), level.AllowInfo())
	if *verbose {
		logger.AddFilters(level.AllowDebug())
	}
	logger.Info("msg", "Starting gonetem daemon", "version", options.VERSION)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	netemServer := server.NewServer()
	go func() {
		socket, err := net.Listen("unix", options.ServerConfig.Listen)
		if err != nil {
			logger.Error("msg", "Unable to listen on socket", "error", err)
			os.Exit(2)
		}

		grpcServer = grpc.NewServer()
		pb.RegisterNetemServer(grpcServer, netemServer)
		err = grpcServer.Serve(socket)
		if err != nil {
			logger.Error("msg", "Error in grpc server", "error", err)
			cancel()
		}
	}()

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}

	logger.Warn("msg", "Received shutdown signal")
	cancel()

	if err := netemServer.Close(); err != nil {
		logger.Error("msg", "Error when close server", "error", err)
	}

	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	// remove unix socket file
	if _, err := os.Stat(options.ServerConfig.Listen); err == nil {
		os.Remove(options.ServerConfig.Listen)
	}
}
