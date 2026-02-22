/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

LNet Ping implementation.
*/
package lnet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
)

const LNET_PROTO_PING_MATCHBITS = 0x8000000000000000
const LNET_PING_MAGIC uint32 = 0x70696E67 // "ping" in ASCII

type PingStatus uint32
type PingFeature uint32

const (
	PING_NI_STATUS_INVALID PingStatus = 0
	PING_NI_STATUS_UP      PingStatus = 0x15aac0de
	PING_NI_STATUS_DOWN    PingStatus = 0xdeadface
)

const (
	PING_FEATURE_INVALID       PingFeature = 0
	PING_FEATURE_PING          PingFeature = 1 << 0
	PING_FEATURE_NI_STATUS     PingFeature = 1 << 1
	PING_FEATURE_RTE_DISABLED  PingFeature = 1 << 2
	PING_FEATURE_MULTI_RAIL    PingFeature = 1 << 3
	PING_FEATURE_DISCOVERY     PingFeature = 1 << 4
	PING_FEATURE_LARGE_ADDRESS PingFeature = 1 << 5
	PING_FEATURE_PRIMARY_LARGE PingFeature = 1 << 6
	PING_FEATURE_METADATA      PingFeature = 1 << 7
)

type PingHeader struct {
	Magic    uint32
	Features uint32
	PID      PID32
	NIDCount uint32
}

type PingResponse struct {
	PingHeader
	NIDStatuses []NIDStatus
}

type NIDStatus struct {
	NID         NID
	Status      PingStatus
	MessageSize uint32
}

func (ping *PingResponse) ToBytes(byteOrder binary.ByteOrder) ([]byte, error) {
	buf := new(bytes.Buffer)
	ping.PingHeader.NIDCount = uint32(len(ping.NIDStatuses))
	if err := binary.Write(buf, byteOrder, ping.PingHeader); err != nil {
		return nil, fmt.Errorf("failed to write PingHeader: %w", err)
	}
	for _, nidStatus := range ping.NIDStatuses {
		nidBytes, err := nidStatus.NID.ToBytes(byteOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to write NIDStatus: %w", err)
		}
		if err := binary.Write(buf, byteOrder, nidBytes); err != nil {
			return nil, fmt.Errorf("failed to write NIDStatus: %w", err)
		}
		if err := binary.Write(buf, byteOrder, nidStatus.Status); err != nil {
			return nil, fmt.Errorf("failed to write NIDStatus: %w", err)
		}
		if err := binary.Write(buf, byteOrder, nidStatus.MessageSize); err != nil {
			return nil, fmt.Errorf("failed to write NIDStatus: %w", err)
		}
	}
	return buf.Bytes(), nil
}

// HandlePing handles a PING command.
func (client *LNetClient) HandlePing(ctx context.Context, remote *RemoteConn, message LNetMessage, command LNetGetCommand) error {
	slog.Info("Handling PING command", "remote", remote, "command", command)
	if command.MatchBits != LNET_PROTO_PING_MATCHBITS {
		return fmt.Errorf("LNET PING has invalid match bits: %d", command.MatchBits)
	}
	if command.PortalIndex != 0 {
		slog.Warn("LNET PING has non-standard portal index", "portalIndex", command.PortalIndex)
	}
	replyMessage := message.GetReply()
	pingResponse := PingResponse{
		PingHeader: PingHeader{
			Magic:    LNET_PING_MAGIC,
			Features: uint32(PING_FEATURE_PING | PING_FEATURE_NI_STATUS),
			PID:      message.DestPID,
		},
		NIDStatuses: make([]NIDStatus, len(client.LocalAddrs)),
	}
	for i, addr := range client.LocalAddrs {
		nid, err := NIDFromAddr(addr, NETWORK_TYPE_TCP, 0, client.Port)
		if err != nil {
			return fmt.Errorf("error creating NID from address: %w", err)
		}
		pingResponse.NIDStatuses[i] = NIDStatus{
			NID:         nid,
			Status:      PING_NI_STATUS_UP,
			MessageSize: 0,
		}
	}
	payload, err := pingResponse.ToBytes(remote.ByteOrder)
	if err != nil {
		return fmt.Errorf("failed to convert ping response to bytes: %w", err)
	}
	replyMessage.SetPayload(remote.ByteOrder, payload)
	return client.SendMessage(ctx, remote, replyMessage)
}
