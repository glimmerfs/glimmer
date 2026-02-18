/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

LNet Command ("message types") handlers.
*/
package lnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
)

type CommandType uint32
type LegacyMessage CommandType

// Compat messages must be handled consistently with Lustre's existing implementation.
const (
	LNET_MSG_ACK CommandType = iota
	LNET_MSG_PUT
	LNET_MSG_GET
	LNET_MSG_REPLY
	LNET_MSG_HELLO
)

// lnet-idl.h
type LNetHeaderEmbed struct {
	DestPID       PID32
	SourcePID     PID32
	MessageType   CommandType
	PayloadLength uint32
}

type LNetMessage struct {
	DestNID   NID
	SourceNID NID
	LNetHeaderEmbed
	LNetCommand any
	Payload     []byte
}

type LNetHandleWire struct {
	InterfaceCookie uint64
	ObjectCookie    uint64
}

// lnet-idl.h

type LNetAckCommand struct {
	DestWMD       LNetHandleWire
	MatchBits     uint64
	MessageLength uint32
}

type LNetGetCommand struct {
	ReturnWMD    LNetHandleWire
	MatchBits    uint64
	PortalIndex  uint32
	SourceOffset uint32
	SinkLength   uint32
}

type LNetPutCommand struct {
	AckWMD      LNetHandleWire
	MatchBits   uint64
	HeaderData  uint64
	PortalIndex uint32
	Offset      uint32
}

type LNetReplyCommand struct {
	DestWMD LNetHandleWire
}

type LNetHelloCommand struct {
	Incarnation uint64
	Type        uint32
}

type CommandHandler func(ctx context.Context, remote RemoteConn, message LNetMessage) error
type CommandRegistry map[CommandType]CommandHandler

func init() {
	if LegacyMessage(LNET_MSG_HELLO) != LegacyMessage(4) {
		panic("LNET_MSG_HELLO value changed, breaking compatibility")
	}
}

// ReadCommand reads an LNet command from the specified remote connection.
func ReadCommand(ctx context.Context, remote *RemoteConn) (LNetMessage, error) {
	_ = ctx
	message := LNetMessage{}
	destNID, err := ReadNID(*remote.Conn, remote.ByteOrder, 0)
	if err != nil {
		return message, err
	}
	sourceNID, err := ReadNID(*remote.Conn, remote.ByteOrder, 0)
	if err != nil {
		return message, err
	}
	var messageTail LNetHeaderEmbed
	if err := binary.Read(*remote.Conn, remote.ByteOrder, &messageTail); err != nil {
		return message, fmt.Errorf("error reading message tail: %w", err)
	}
	slog.Info("received LNET message header", "destNID", destNID, "sourceNID", sourceNID, "messageTail", messageTail, "remote", remote)
	message = LNetMessage{DestNID: destNID, SourceNID: sourceNID, LNetHeaderEmbed: messageTail}
	switch message.MessageType {
	case LNET_MSG_ACK:
		message.LNetCommand = &LNetAckCommand{}
	case LNET_MSG_PUT:
		message.LNetCommand = &LNetPutCommand{}
	case LNET_MSG_GET:
		message.LNetCommand = &LNetGetCommand{}
	case LNET_MSG_REPLY:
		message.LNetCommand = &LNetReplyCommand{}
	case LNET_MSG_HELLO:
		message.LNetCommand = &LNetHelloCommand{}
	default:
		slog.Warn("Unsupported LNET message type", "messageType", message.MessageType)
		return message, fmt.Errorf("unsupported LNET message type: %d", message.MessageType)
	}
	if err := binary.Read(*remote.Conn, remote.ByteOrder, message.LNetCommand); err != nil {
		return message, fmt.Errorf("error reading LNET %v message: %w", message.MessageType, err)
	}
	if message.PayloadLength > 0 {
		message.Payload = make([]byte, message.PayloadLength)
		if _, err := (*remote.Conn).Read(message.Payload); err != nil {
			return message, fmt.Errorf("error reading LNET message payload: %w", err)
		}
	}
	return message, nil
}

// SendCommand sends an LNet command to the specified remote connection.
func SendCommand(ctx context.Context, remote *RemoteConn, message LNetMessage) error {
	_ = ctx
	if message.LNetCommand == nil {
		return fmt.Errorf("cannot send LNET message with nil command")
	}
	return nil
}

func HandleGet(ctx context.Context, remote RemoteConn, message LNetMessage) error {
	_ = ctx
	slog.Info("Handling GET command", "remote", remote, "message", message)
	return nil
}
