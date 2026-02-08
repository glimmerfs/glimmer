/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0
*/
package lnet

import (
	"context"
	"encoding/binary"
	"errors"
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

type RemoteConn struct {
	Conn      *net.Conn
	ByteOrder binary.ByteOrder
	Protocol  ProtocolMagic
}

// NewLNetClient creates a new LNetClient with default settings.
func NewLNetClient() LNetClient {
	return LNetClient{ByteOrder: DEFAULT_BYTE_ORDER, Port: DEFAULT_PORT}
}

// WithPort returns a copy of the LNetClient with the specified port.
func (client LNetClient) WithPort(port uint16) LNetClient {
	// Not referring to client via *LNetClient gives us a copy
	client.Port = port
	return client
}

// Get the opposite of the current byte order
func (client *LNetClient) getOppositeByteOrder() (binary.ByteOrder, error) {
	var nativeBuff [4]byte
	client.ByteOrder.PutUint32(nativeBuff[:], 0x01020304)
	switch nativeBuff {
	case [4]byte{0x04, 0x03, 0x02, 0x01}:
		// Native is little endian, so other must be big endian
		return binary.BigEndian, nil
	case [4]byte{0x01, 0x02, 0x03, 0x04}:
		// Native is big endian, so other must be little endian
		return binary.LittleEndian, nil
	default:
		return nil, fmt.Errorf("unknown native byte order")
	}
}

func (client *LNetClient) Negotiate(ctx context.Context, conn net.Conn) (RemoteConn, error) {
	_ = ctx
	_ = conn
	var acceptorMagic ProtocolMagic
	remote := RemoteConn{Conn: &conn, ByteOrder: client.ByteOrder}
	err := binary.Read(conn, client.ByteOrder, &acceptorMagic)
	if err != nil {
		return remote, fmt.Errorf("failed to read acceptor magic: %w", err)
	}
	switch acceptorMagic {
	case PROTO_MAGIC_ACCEPTOR_REV:
		slog.Info("Detected reverse byte order from remote, switching byte order for this connection")
		remote.ByteOrder, err = client.getOppositeByteOrder()
		if err != nil {
			return remote, fmt.Errorf("failed to determine opposite byte order: %w", err)
		}
	case PROTO_MAGIC_GENERIC:
		return remote, fmt.Errorf("Generic protocol not supported by LNetClient yet")
	case PROTO_MAGIC_ACCEPTOR:
		slog.Debug("Received valid acceptor magic from remote, proceeding with negotiation")
	default:
		return remote, fmt.Errorf("invalid acceptor magic: expected 0x%08x, got 0x%08x", PROTO_MAGIC_ACCEPTOR, acceptorMagic)
	}
	// TODO: detect actual protocol (e.g., TCP)
	return remote, errors.ErrUnsupported
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

	remote, err := client.Negotiate(ctx, conn)
	if err != nil {
		slog.Error("LNetClient negotiation failed", "error", err, "remote", remote)
		return
	}
}
