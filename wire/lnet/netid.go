/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Network identifiers and methods.
*/
package lnet

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"regexp"
	"strconv"
)

type NID interface {
	String() string
	NetAddr() netip.Addr
	IsAny() bool
}

type NIDHeader struct {
	Size         uint8       // len(RawNID)-8: 0 for NID64, 12 for ExtendedNID
	Type         NetworkType // 0xFF if wildcard (matches any NID)
	NetworkIndex uint16
}

// NID64 is a 64-bit NID that can fit a 32-bit address (e.g., IPv4)
type NID64 struct {
	NIDHeader
	Addr [1]uint32 // One IPv4 address fits into 4 bytes
	// Non-standard Port field for userland flexibility, not part of Lustre's NID64 structure
	Port uint16
}

// ExtendedNID is a 160-bit NID that can fit a 128-bit address (e.g., IPv6)
type ExtendedNID struct {
	NIDHeader
	Addr [4]uint32 // a NID64 is basically this struct with only Addr [1]uint32
	// Non-standard Port field for userland flexibility, not part of Lustre's NID64 structure
	Port uint16
}

// Ensure that both NID64 and ExtendedNID implement the NID interface
var _ NID = (*NID64)(nil)
var _ NID = (*ExtendedNID)(nil)

// NIDFromAddr creates a NID from a netip.Addr
func NIDFromAddr(addr netip.Addr, netType NetworkType, netNum uint16, portNum uint16) (NID, error) {
	if addr.IsUnspecified() {
		return AnyNID, nil
	}
	addrBytes := addr.AsSlice()
	if portNum == 0 {
		portNum = DEFAULT_PORT
	}
	size := uint8(len(addrBytes) - 4)
	header := NIDHeader{Size: size, Type: netType, NetworkIndex: netNum}
	if addr.Is4() {
		blocks := [1]uint32{DEFAULT_BYTE_ORDER.Uint32(addr.AsSlice())}
		return NID64{NIDHeader: header, Addr: blocks, Port: portNum}, nil
	} else if addr.Is6() {
		var blocks [4]uint32
		for i := range 4 {
			blocks[i] = DEFAULT_BYTE_ORDER.Uint32(addrBytes[i*4 : (i+1)*4])
		}
		return ExtendedNID{NIDHeader: header, Addr: blocks, Port: portNum}, nil
	}
	return nil, fmt.Errorf("unsupported address type for NID: %s", addr)
}

var ValidNIDExpr *regexp.Regexp = regexp.MustCompile(`^([0-9a-fA-F:.]+)@([a-zA-Z0-9]+[a-zA-Z])(\d+)(?:#(\d{1,5}))?$`)

// ParseNID parses a string of the form "ADDRESS@PROTOCOL#PORT" into a NID.
func ParseNID(s string) (NID, error) {
	if s == "any" || s == "*" {
		return AnyNID, nil
	}
	matches := ValidNIDExpr.FindStringSubmatch(s)
	var addrStr, protoStr, netNumStr, portStr string
	if matches == nil {
		return nil, fmt.Errorf("invalid NID format: %s", s)
	}
	if len(matches) == 4 {
		addrStr, protoStr, netNumStr = matches[1], matches[2], matches[3]
	} else if len(matches) == 5 {
		addrStr, protoStr, netNumStr, portStr = matches[1], matches[2], matches[3], matches[4]
	} else {
		return nil, fmt.Errorf("unexpected regex match length: %d", len(matches))
	}
	networkType, err := NetworkTypeFromString(protoStr)
	if err != nil {
		return nil, fmt.Errorf("invalid network type in NID: %w", err)
	}
	networkNum, err := strconv.ParseUint(netNumStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid network number in NID: %w", err)
	}
	// ParseUint always returns a uint64
	portNum := uint64(DEFAULT_PORT)
	if portStr != "" {
		portNum, err = strconv.ParseUint(portStr, 10, 16)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid port number in NID: %w", err)
	}
	addr, err := netip.ParseAddr(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address in NID: %w", err)
	}
	return NIDFromAddr(addr, networkType, uint16(networkNum), uint16(portNum))
}

// ReadNID reads a NID from a reader (e.g., socket)
// Note that Lustre uses protocol version to determine read length
func ReadNID(reader io.Reader, byteOrder binary.ByteOrder) (NID, error) {
	var header NIDHeader
	err := binary.Read(reader, byteOrder, &header)
	if err != nil {
		return nil, fmt.Errorf("failed to read NID header: %w", err)
	}

	if header.Type == NETWORK_TYPE_ANY {
		var addr [1]uint32
		err := binary.Read(reader, byteOrder, &addr)
		if err != nil {
			if err == io.EOF {
				// If we hit EOF while reading the address, we can still treat this as AnyNID
				// We do want to still raise for io.EOFUnexpected though
				slog.Warn("EOF reached while reading AnyNID address", "error", err)
				return AnyNID, nil
			}
			return nil, fmt.Errorf("failed to read NID address: %w", err)
		}
		return AnyNID, nil
	}

	switch header.Size {
	case 0:
		var addr [1]uint32
		err := binary.Read(reader, byteOrder, &addr)
		if err != nil {
			return nil, fmt.Errorf("failed to read NID64 address: %w", err)
		}
		return NID64{NIDHeader: header, Addr: addr, Port: DEFAULT_PORT}, nil
	case 2:
		// TODO implement this case later
		// WARNING: Cannot use with Lustre peers
		return nil, fmt.Errorf("IPv4 + Port not yet supported")
	case 12:
		// YAGNI: We MAY need to handle 1-11 size for extended nid
		var addr [4]uint32
		err := binary.Read(reader, byteOrder, &addr)
		if err != nil {
			return nil, fmt.Errorf("failed to read ExtendedNID address: %w", err)
		}
		return ExtendedNID{NIDHeader: header, Addr: addr, Port: DEFAULT_PORT}, nil
	case 14:
		// TODO implement this case later
		// WARNING: Cannot use with Lustre peers
		return nil, fmt.Errorf("IPv6 + Port not yet supported")
	}
	return nil, fmt.Errorf("unsupported NID size: %d", header.Size)
}

// Extracts the lowest 4 bytes of the address from a NID64, assuming it's an IPv4 address.
func (nid NID64) AddrBytes() [4]byte {
	var bytes [4]byte
	DEFAULT_BYTE_ORDER.PutUint32(bytes[:4], nid.Addr[0])
	return bytes
}

// Extracts the packed 128-bit address into 16 bytes
func (enid ExtendedNID) AddrBytes() [16]byte {
	var bytes []byte
	for _, addrValue := range enid.Addr {
		DEFAULT_BYTE_ORDER.AppendUint32(bytes, addrValue)
	}
	return [16]byte(bytes)
}

// Converts the NID64 to a netip.Addr, assuming it's an IPv4 address.
func (nid NID64) NetAddr() netip.Addr {
	return netip.AddrFrom4(nid.AddrBytes())
}

// Converts the ExtendedNID to a netip.Addr, assuming it's an IPv6 address.
func (enid ExtendedNID) NetAddr() netip.Addr {
	return netip.AddrFrom16(enid.AddrBytes())
}

func (nid NID64) String() string {
	return fmt.Sprintf("%s@%s%d#%d", nid.NetAddr().String(), nid.Type.String(), nid.NetworkIndex, nid.Port)
}

func (enid ExtendedNID) String() string {
	return fmt.Sprintf("%s@%s%d#%d", enid.NetAddr().String(), enid.Type.String(), enid.NetworkIndex, enid.Port)
}

// IsAny reports whether the NID is a wildcard (matches any NID).
func (nid NID64) IsAny() bool {
	return nid.Type == NETWORK_TYPE_ANY
}

// IsAny reports whether the ExtendedNID is a wildcard (matches any NID).
func (enid ExtendedNID) IsAny() bool {
	return enid.Type == NETWORK_TYPE_ANY
}

// In Lustre, the entire structure is set to ~0 (all bits set)
// (but we really only care about Type being set properly)
var AnyNID NID = NID64{NIDHeader: NIDHeader{Size: 0xFF, Type: NETWORK_TYPE_ANY, NetworkIndex: 0xFF}, Addr: [1]uint32{0xFFFFFFFF}, Port: DEFAULT_PORT}
