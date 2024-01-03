package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/mroy31/gonetem/internal/options"
	pb "github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/server"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	grpcServer *grpc.Server = nil
	verbose                 = flag.Bool("verbose", false, "Display more messages")
	conf                    = flag.String("conf-file", options.SERVER_CONFIG_FILE, "Configuration path")
	logFile                 = flag.String("log-file", "", "Path of the log file (default: stdout)")
)

func main() {
	flag.Parse()

	// init log
	logWriter := os.Stderr
	if *logFile != "" {
		f, err := os.Create(*logFile)
		if err != nil {
			logrus.Fatalf("Unable to create log file %s: %v", *logFile, err)
		}
		defer f.Close()

		logWriter = f
	}
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(logWriter)
	logrus.SetLevel(logrus.InfoLevel)
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.Info("Starting gonetem daemon - version " + options.VERSION)

	// parse config
	options.InitServerConfig()
	if _, err := os.Stat(*conf); os.IsNotExist(err) {
		logrus.Warnf("Config file %s not exit, skip it", *conf)
	} else {
		if err := options.ParseServerConfig(*conf); err != nil {
			logrus.Fatalf("Unable to parse config file %s: %v", *conf, err)
		}
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	netemServer := server.NewServer()
	go func() {
		var serverOptions []grpc.ServerOption = make([]grpc.ServerOption, 0)

		logrus.Infof("TLS enabled: %t", options.ServerConfig.Tls.Enabled)
		if options.ServerConfig.Tls.Enabled {
			tlsCredentials, err := options.LoadServerTLSCredentials()
			if err != nil {
				logrus.Errorf("cannot load TLS credentials: %v", err)
				os.Exit(2)
			}
			serverOptions = append(serverOptions, grpc.Creds(tlsCredentials))
		}

		grpcServer = grpc.NewServer(serverOptions...)
		pb.RegisterNetemServer(grpcServer, netemServer)

		logrus.Infof("Listen on: %s", options.ServerConfig.Listen)
		socket, err := net.Listen("tcp", options.ServerConfig.Listen)
		if err != nil {
			logrus.Errorf("unable to listen on socket: %v", err)
			os.Exit(2)
		}

		err = grpcServer.Serve(socket)
		if err != nil {
			logrus.Errorf("Error in grpc server: %v", err)
			cancel()
		}
	}()

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}

	logrus.Warn("Received shutdown signal")
	cancel()

	if err := netemServer.Close(); err != nil {
		logrus.Errorf("Error when close server %v", err)
	}

	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	// remove unix socket file
	if _, err := os.Stat(options.ServerConfig.Listen); err == nil {
		os.Remove(options.ServerConfig.Listen)
	}
}
