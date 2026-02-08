/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Network identifiers and methods.
*/
package lnet

import (
	"fmt"
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

var PROTO_MAGIC_ACCEPTOR_LE = [4]byte{0x00, 0x71, 0xce, 0xac} // PROTO_MAGIC_ACCEPTOR encoded as little-endian bytes
var PROTO_MAGIC_ACCEPTOR_BE = [4]byte{0xac, 0xce, 0x71, 0x00} // PROTO_MAGIC_ACCEPTOR encoded as big-endian bytes

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
