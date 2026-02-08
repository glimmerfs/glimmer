/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0
*/
package lnet

import (
	"context"
	"fmt"
	"log/slog"
	"net"
)

type LNetServer struct {
	// Underlying client for handling connections and messages
	Client LNetClient
	// Configuration for incoming connections
	ListenConfig net.ListenConfig
}

func NewLNetServer() *LNetServer {
	return &LNetServer{Client: NewLNetClient()}
}

// Listen to connections and dispatch valid connections to handlers
func (server *LNetServer) Listen(ctx context.Context) error {
	// YAGNI: support more than just tcp? like o2ib?
	listener, err := server.ListenConfig.Listen(ctx, "tcp", fmt.Sprintf(":%d", server.Client.Port))
	if err != nil {
		return err
	}
	// Ensure the listener is closed
	// This can be called multiple times
	closeListener := func() {
		slog.Debug("LNetServer listener shutting down")
		err := listener.Close()
		if err != nil {
			slog.Warn("error closing listener", "error", err)
		}
	}
	defer closeListener()
	slog.Info("LNetServer listening", "port", server.Client.Port)
	context.AfterFunc(ctx, closeListener)

	for {
		slog.Debug("LNetServer waiting for connection")
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				// Context was cancelled, exit gracefully
				return nil
			}
			slog.Error("Accept Error", "error", err)
			return err
		}
		go server.Client.handleConnection(ctx, conn)
	}
}
