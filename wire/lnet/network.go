/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Network identifiers and methods.
*/
package lnet

import (
	"encoding/binary"
)

// we use native-endian on our side (receiver). on the remote side, we need to detect and do flipping if needed.
// NOTE: this is going to be LittleEndian in almost all cases
var DEFAULT_BYTE_ORDER interface {
	binary.ByteOrder
	binary.AppendByteOrder
} = binary.NativeEndian

// LNet traditionally expects the same port across the cluster
// but we need to support multiple ports in userland
// We allow a new port to be specified by appending #PORT to the NID string
// e.g., "192.168.105.12@tcp0#9881"
var DEFAULT_PORT uint16 = 988
