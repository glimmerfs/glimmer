/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

LNet Command ("message types") handlers.
*/
package lnet

import (
	"bytes"
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

type CommandHandler func(ctx context.Context, remote *RemoteConn, message LNetMessage) error
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

func (message *LNetMessage) ToBytes(byteOrder binary.ByteOrder) ([]byte, error) {
	buf := new(bytes.Buffer)
	if data, err := message.DestNID.ToBytes(byteOrder); err != nil {
		return nil, fmt.Errorf("failed to write NID64: %w", err)
	} else {
		buf.Write(data)
	}
	if data, err := message.SourceNID.ToBytes(byteOrder); err != nil {
		return nil, fmt.Errorf("failed to write NID64: %w", err)
	} else {
		buf.Write(data)
	}
	message.LNetHeaderEmbed.PayloadLength = uint32(len(message.Payload))
	if err := binary.Write(buf, byteOrder, message.LNetHeaderEmbed); err != nil {
		return nil, fmt.Errorf("failed to write LNetHeaderEmbed: %w", err)
	}
	if message.LNetCommand == nil {
		return nil, fmt.Errorf("cannot write LNet message with nil command")
	}
	if err := binary.Write(buf, byteOrder, message.LNetCommand); err != nil {
		return nil, fmt.Errorf("failed to write LNetCommand: %w", err)
	}
	if err := binary.Write(buf, byteOrder, message.Payload); err != nil {
		return nil, fmt.Errorf("failed to write Payload: %w", err)
	}
	return buf.Bytes(), nil
}

// GetReply returns the REPLY message for the given GET message.
func (message *LNetMessage) GetReply() LNetMessage {
	command := message.LNetCommand.(*LNetGetCommand)
	return LNetMessage{
		DestNID:   message.SourceNID,
		SourceNID: message.DestNID,
		LNetHeaderEmbed: LNetHeaderEmbed{
			DestPID:       message.SourcePID,
			SourcePID:     message.DestPID,
			MessageType:   LNET_MSG_REPLY,
			PayloadLength: 0,
		},
		LNetCommand: &LNetReplyCommand{
			DestWMD: command.ReturnWMD,
		},
	}
}

func (message *LNetMessage) SetPayload(byteOrder binary.ByteOrder, payload any) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, byteOrder, payload); err != nil {
		slog.Error("failed to write payload", "error", err, "payload", payload)
		return
	}
	slog.Info("Set payload", "payload", payload, "length", buf.Len())
	message.Payload = buf.Bytes()
	message.PayloadLength = uint32(buf.Len())
}

func (client *LNetClient) HandleGet(ctx context.Context, remote *RemoteConn, message LNetMessage) error {
	slog.Info("Handling GET command", "remote", remote, "message", message)
	command := message.LNetCommand.(*LNetGetCommand)
	if command.MatchBits == LNET_PROTO_PING_MATCHBITS {
		return client.HandlePing(ctx, remote, message, *command)
	}
	return nil
}
