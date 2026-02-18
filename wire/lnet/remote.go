/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0
*/
package lnet

import (
	"encoding/binary"
	"net"
)

type RemoteConn struct {
	Conn      *net.Conn
	ByteOrder binary.ByteOrder
	Protocol  ProtocolMagic
	NID       NID
}
