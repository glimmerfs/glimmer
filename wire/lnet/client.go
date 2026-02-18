/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0
*/
package lnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
)

// A client for communication via LNet.
// This is intended to be somewhat compatible with Lustre's existing LNet implementation
type LNetClient struct {
	CompatMode bool             // Stricter communication with Lustre
	ByteOrder  binary.ByteOrder // we use native-endian on our side (receiver). on the remote side, we need to detect and do flipping if needed.
	LocalAddrs []netip.Addr     // Known local IPs (validates destinations)
	// Port to listen on for incoming LNet connections
	// As a special change, we allow NIDs to have a #PORT suffix to change the default
	Port uint16
	// Command registry for handling different LNet message types
	Commands CommandRegistry
}

// NewLNetClient creates a new LNetClient with default settings.
func NewLNetClient() LNetClient {
	client := LNetClient{ByteOrder: DEFAULT_BYTE_ORDER, Port: DEFAULT_PORT}
	client.Commands = make(CommandRegistry)
	client.Commands[LNET_MSG_GET] = HandleGet
	return client
}

// WithPort returns a copy of the LNetClient with the specified port.
func (client LNetClient) WithPort(port uint16) LNetClient {
	// Not referring to client via *LNetClient gives us a copy
	client.Port = port
	return client
}

func (client *LNetClient) handleCommands(ctx context.Context, remote *RemoteConn) error {
	_ = ctx
	for {
		var messageHeader KSockMessageHeader
		if err := binary.Read(*remote.Conn, remote.ByteOrder, &messageHeader); err != nil {
			slog.Error("error reading message header", "error", err, "remote", remote)
			return err
		}
		switch messageHeader.Type {
		case KSOCK_MSG_NOOP:
			slog.Info("received NOOP message", "remote", remote)
		case KSOCK_MSG_LNET:
			slog.Info("received LNET message", "remote", remote)
			if messageHeader.Checksum != 0 {
				slog.Warn("LNET message has non-zero checksum, which is unsupported", "checksum", messageHeader.Checksum, "remote", remote)
			}
			message, err := ReadCommand(ctx, remote)
			if err != nil {
				slog.Error("error reading LNET message", "error", err, "remote", remote)
				return err
			}
			handler, ok := client.Commands[message.MessageType]
			if !ok {
				slog.Warn("no handler registered for message type, ignoring message", "messageType", message.MessageType, "remote", remote)
				continue
			}
			if err := handler(ctx, *remote, message); err != nil {
				slog.Error("error handling message", "error", err, "messageType", message.MessageType, "remote", remote)
				return err
			}
		default:
			return fmt.Errorf("unsupported message type: %d", messageHeader.Type)
		}
	}
}

func (client *LNetClient) handleConnection(ctx context.Context, conn net.Conn) {
	_ = ctx
	defer func() {
		err := conn.Close()
		if err != nil {
			slog.Warn("error closing connection", "error", err, "remote", conn.RemoteAddr())
		}
	}()
	slog.Info("LNetClient accepted connection", "remote", conn.RemoteAddr())

	remote := RemoteConn{Conn: &conn, ByteOrder: client.ByteOrder}
	err := Negotiate(ctx, &remote)
	if err != nil {
		slog.Error("LNetClient negotiation failed", "error", err, "remote", remote)
		return
	}
	slog.Info("LNetClient negotiation succeeded", "remote", remote)

	err = client.handleCommands(ctx, &remote)
	if err != nil {
		slog.Error("LNetClient command handling failed", "error", err, "remote", remote)
		panic(err) // abort to limit traffic for now
		return
	}
}
