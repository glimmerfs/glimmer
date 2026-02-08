/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

LNet Command ("message types") handlers.
*/
package lnet

import (
	"context"
	"net"
)

type CommandType uint32
type LegacyMessage CommandType

// Compat messages must be handled consistently with Lustre's existing implementation.
const (
	LNET_MSG_ACK LegacyMessage = iota
	LNET_MSG_PUT
	LNET_MSG_GET
	LNET_MSG_REPLY
	LNET_MSG_HELLO
)

type LNetMessage struct {
	DestNID       NID
	SourceNID     NID
	DestPID       PID32
	SourcePID     PID32
	Command       CommandType
	PayloadLength uint32
	Payload       []byte
}

type CommandHandler func(ctx context.Context, conn net.Conn, message LNetMessage) error
type CommandRegistry map[CommandType]CommandHandler

func init() {
	if LNET_MSG_HELLO != LegacyMessage(4) {
		panic("LNET_MSG_HELLO value changed, breaking compatibility")
	}
}
