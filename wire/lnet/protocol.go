/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Network protocols and types.
*/
package lnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
)

// PID32 is a 32-bit process identifier used in LNet messages.
type PID32 uint32

type NetworkType uint8

type ProtocolMagic uint32

// nidstr.h
const (
	NETWORK_TYPE_INVALID NetworkType = iota
	NETWORK_TYPE_TCP     NetworkType = 2 // SOCKLND
	NETWORK_TYPE_O2IB    NetworkType = 5 // o2ib
	NETWORK_TYPE_LO      NetworkType = 9 // loopback
	NETWORK_TYPE_ANY     NetworkType = 0xFF
)

// lnet-types.h
const (
	PID_LUSTRE   PID32 = 12345      // yeah, that's the actual value
	PID_GLIMMER  PID32 = 54321      // For GlimmerFS processes
	PID_USERLAND PID32 = 0x80000000 // For userland processes (bit flag)
	PID_RESERVED PID32 = 0xF0000000 // Reserved bits
)

// lnet-idl.h
const (
	// All protocols use this to start negotiation
	PROTO_MAGIC_ACCEPTOR ProtocolMagic = 0xacce7100
	// Reverse byte order (to detect endianness)
	PROTO_MAGIC_ACCEPTOR_REV ProtocolMagic = 0x0071ceac
	// Unified LND protocol magic
	PROTO_MAGIC_GENERIC ProtocolMagic = 0x45726963
	// Normal TCP/SOCKLND magic
	PROTO_MAGIC_TCP ProtocolMagic = 0xeebc0ded
)

// socklnd.h
const (
	KSOCK_MSG_NOOP uint32 = 0xc0
	KSOCK_MSG_LNET uint32 = 0xc1
)

func NetworkTypeFromString(s string) (NetworkType, error) {
	s = strings.ToLower(s)
	switch s {
	case "tcp":
		return NETWORK_TYPE_TCP, nil
	case "o2ib":
		return NETWORK_TYPE_O2IB, nil
	case "lo":
		return NETWORK_TYPE_LO, nil
	}
	return NETWORK_TYPE_INVALID, fmt.Errorf("unsupported network type: %s", s)
}

func (netType NetworkType) String() string {
	switch netType {
	case NETWORK_TYPE_TCP:
		return "tcp"
	case NETWORK_TYPE_O2IB:
		return "o2ib"
	case NETWORK_TYPE_LO:
		return "lo"
	case NETWORK_TYPE_ANY:
		return "any"
	default:
		return fmt.Sprintf("unknown(%d)", netType)
	}
}

// socklnd.h
type KSockMessageHeader struct {
	Type            uint32
	Checksum        uint32
	ZeroCopyCookies [2]uint64
}

type helloResponseCommonTail struct {
	SourcePID         PID32
	DestPID           PID32
	SourceIncarnation uint64
	DestIncarnation   uint64
	ConnType          uint32
	NIPs              uint32
	// IPs uint32[] Unsupported (zero length array)
}

type helloResponse struct {
	Magic        ProtocolMagic
	ProtoVersion uint32
	SourceNID    RawExtendedNID
	DestNID      RawExtendedNID
	helloResponseCommonTail
}

type helloResponseV2 struct {
	Magic        ProtocolMagic
	ProtoVersion uint32
	SourceNID    RawNID64
	DestNID      RawNID64
	helloResponseCommonTail
}

// ProtocolUpgrade switches to a better protocol as part of Negotiation
// This handles ksock_hello_msg from socklnd.h
func ProtocolUpgrade(ctx context.Context, remote *RemoteConn) error {
	_ = ctx
	var protocolMagic ProtocolMagic
	if err := binary.Read(*remote.Conn, remote.ByteOrder, &protocolMagic); err != nil {
		return fmt.Errorf("failed to read protocol magic: %w", err)
	}
	switch protocolMagic {
	case PROTO_MAGIC_GENERIC:
		slog.Info("Remote supports unified protocol, switching to TCP protocol")
		remote.Protocol = PROTO_MAGIC_TCP
	case PROTO_MAGIC_TCP:
		slog.Info("Remote requests TCP/SOCKLND protocol, switching to TCP protocol")
		remote.Protocol = PROTO_MAGIC_TCP
	default:
		return fmt.Errorf("unsupported protocol magic: got 0x%08x", protocolMagic)
	}

	var protocolVersion uint32
	if err := binary.Read(*remote.Conn, remote.ByteOrder, &protocolVersion); err != nil {
		return fmt.Errorf("failed to read protocol version: %w", err)
	}

	handleCommon := func() (helloResponseCommonTail, error) {
		var commonTail helloResponseCommonTail
		if err := binary.Read(*remote.Conn, remote.ByteOrder, &commonTail); err != nil {
			return helloResponseCommonTail{}, fmt.Errorf("failed to read common tail: %w", err)
		}
		if commonTail.NIPs != 0 {
			return helloResponseCommonTail{}, fmt.Errorf("unsupported non-zero NIPs value: %d", commonTail.NIPs)
		}
		incarnation := commonTail.SourceIncarnation
		commonTail.DestIncarnation = incarnation
		commonTail.SourceIncarnation = rand.Uint64()
		commonTail.DestPID = commonTail.SourcePID
		// commonTail.SourcePID = PID_GLIMMER | PID_USERLAND
		if commonTail.ConnType == 2 {
			commonTail.ConnType = 3
		}
		return commonTail, nil
	}

	switch protocolVersion {
	case 2, 3:
		slog.Info("Remote is using protocol version 2/3, expecting hello message with NID64 format", "version", protocolVersion)
		var rawSourceNID64 RawNID64
		var rawDestNID64 RawNID64
		if err := binary.Read(*remote.Conn, remote.ByteOrder, &rawSourceNID64); err != nil {
			return fmt.Errorf("failed to read source NID in protocol version %d: %w", protocolVersion, err)
		}
		if err := binary.Read(*remote.Conn, remote.ByteOrder, &rawDestNID64); err != nil {
			return fmt.Errorf("failed to read destination NID in protocol version %d: %w", protocolVersion, err)
		}
		commonTail, err := handleCommon()
		if err != nil {
			return err
		}
		response := helloResponseV2{
			Magic:        protocolMagic,
			ProtoVersion: protocolVersion,
			// swap nids in response
			SourceNID:               rawDestNID64,
			DestNID:                 rawSourceNID64,
			helloResponseCommonTail: commonTail,
		}
		if err := binary.Write(*remote.Conn, remote.ByteOrder, &response); err != nil {
			return fmt.Errorf("failed to write hello response in protocol version 2: %w", err)
		}
	case 4:
		slog.Info("Remote is using protocol version 4, expecting hello message with ExtendedNID format")
		var rawSourceENid RawExtendedNID
		var rawDestENid RawExtendedNID
		if err := binary.Read(*remote.Conn, remote.ByteOrder, &rawSourceENid); err != nil {
			return fmt.Errorf("failed to read source NID in protocol version 4: %w", err)
		}
		if err := binary.Read(*remote.Conn, remote.ByteOrder, &rawDestENid); err != nil {
			return fmt.Errorf("failed to read destination NID in protocol version 4: %w", err)
		}
		commonTail, err := handleCommon()
		if err != nil {
			return err
		}
		response := helloResponse{
			Magic:        remote.Protocol,
			ProtoVersion: protocolVersion,
			// swap nids in response
			SourceNID:               rawDestENid,
			DestNID:                 rawSourceENid,
			helloResponseCommonTail: commonTail,
		}
		if err := binary.Write(*remote.Conn, remote.ByteOrder, &response); err != nil {
			return fmt.Errorf("failed to write hello response in protocol version 4: %w", err)
		}
	default:
		return fmt.Errorf("unsupported protocol version: %d", protocolVersion)
	}
	return nil
}

// Negotiate handles initial protocol negotiation with the remote peer.
func Negotiate(ctx context.Context, remote *RemoteConn) error {
	var acceptorMagic ProtocolMagic
	if err := binary.Read(*remote.Conn, remote.ByteOrder, &acceptorMagic); err != nil {
		return fmt.Errorf("failed to read acceptor magic: %w", err)
	}
	switch acceptorMagic {
	case PROTO_MAGIC_ACCEPTOR_REV:
		slog.Info("Detected reverse byte order from remote, switching byte order for this connection")
		remote.ByteOrder = GetOppositeByteOrder(remote.ByteOrder)
	case PROTO_MAGIC_GENERIC:
		return fmt.Errorf("Generic protocol not supported by LNetClient yet")
	case PROTO_MAGIC_ACCEPTOR:
		slog.Debug("Received valid acceptor magic from remote, proceeding with negotiation")
	default:
		return fmt.Errorf("invalid acceptor magic: expected 0x%08x, got 0x%08x", PROTO_MAGIC_ACCEPTOR, acceptorMagic)
	}
	var acceptorVersion uint32
	if err := binary.Read(*remote.Conn, remote.ByteOrder, &acceptorVersion); err != nil {
		return fmt.Errorf("failed to read acceptor version: %w", err)
	}

	var sourceNID NID
	var err error
	switch acceptorVersion {
	case 1:
		slog.Info("Remote is using supported acceptor version 1, proceeding with negotiation")
		sourceNID, err = ReadNID(*remote.Conn, remote.ByteOrder, acceptorVersion)
		if err != nil {
			return fmt.Errorf("failed to read source NID: %w", err)
		}
		if _, ok := sourceNID.(NID64); !ok {
			slog.Warn("Remote sent non-NID64 source NID, which may not be compatible with Lustre peers", "remote_nid", sourceNID)
		}
	case 2:
		slog.Info("Remote is using supported acceptor version 2, proceeding with negotiation")
		sourceNID, err = ReadNID(*remote.Conn, remote.ByteOrder, acceptorVersion)
		if err != nil {
			return fmt.Errorf("failed to read source NID: %w", err)
		}
		if _, ok := sourceNID.(ExtendedNID); !ok {
			slog.Warn("Remote sent non-ExtendedNID source NID in acceptor version 2, which may not be compatible with Lustre peers", "remote_nid", sourceNID)
		}
	default:
		return fmt.Errorf("unsupported acceptor version: expected 1, got %d", acceptorVersion)
	}
	remote.NID = sourceNID
	return ProtocolUpgrade(ctx, remote)
}
