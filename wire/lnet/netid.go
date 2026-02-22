/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Network identifiers and methods.
*/
package lnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strconv"
)

type NID interface {
	String() string
	NetAddr() netip.Addr
	IsAny() bool
	ToBytes(binary.ByteOrder) ([]byte, error)
}

type NIDHeader struct {
	Size         uint8       // len(RawNID)-8: 0 for NID64, 12 for ExtendedNID
	Type         NetworkType // 0xFF if wildcard (matches any NID)
	NetworkIndex uint16
}

type RawNID64 uint64

// NID64 is a 64-bit NID that can fit a 32-bit address (e.g., IPv4)
type NID64 struct {
	NIDHeader
	Addr [1]uint32 // One IPv4 address fits into 4 bytes
	// Non-standard Port field for userland flexibility, not part of Lustre's NID64 structure
	Port uint16
}

type RawExtendedNID struct {
	NIDHeader
	Addr [4]uint32
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

func (rawNid RawNID64) ToNID64() NID64 {
	header := NIDHeader{}
	addr0 := uint32(rawNid & 0xFFFFFFFF)
	header.Size = uint8((rawNid >> 56) & 0xFF)
	header.Type = NetworkType((rawNid >> 48) & 0xFF)
	header.NetworkIndex = uint16((rawNid >> 32) & 0xFFFF)

	if header.Type == NETWORK_TYPE_ANY {
		return AnyNID.(NID64)
	}
	return NID64{NIDHeader: header, Addr: [1]uint32{addr0}, Port: DEFAULT_PORT}
}

func (rawNid RawExtendedNID) ToExtendedNID() ExtendedNID {
	return ExtendedNID{NIDHeader: rawNid.NIDHeader, Addr: rawNid.Addr, Port: DEFAULT_PORT}
}

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
func ReadNID(reader io.Reader, byteOrder binary.ByteOrder, versionHint uint32) (NID, error) {
	_ = versionHint // This may be required in better ExtendedNID cases
	// TODO: use RawNID64
	var rawHeader uint64
	var header NIDHeader
	err := binary.Read(reader, byteOrder, &rawHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to read NID header: %w", err)
	}

	addr0 := uint32(rawHeader & 0xFFFFFFFF)
	header.Size = uint8((rawHeader >> 56) & 0xFF)
	header.Type = NetworkType((rawHeader >> 48) & 0xFF)
	header.NetworkIndex = uint16((rawHeader >> 32) & 0xFFFF)

	if header.Type == NETWORK_TYPE_ANY {
		return AnyNID, nil
	}

	// FIXME: right now, we just rely on Size, but ENID MAY actually have size defined later
	switch header.Size {
	case 0:
		return NID64{NIDHeader: header, Addr: [1]uint32{addr0}, Port: DEFAULT_PORT}, nil
	case 2:
		// TODO implement this case later
		// WARNING: Cannot use with Lustre peers
		return nil, fmt.Errorf("IPv4 + Port not yet supported")
	case 12:
		// YAGNI: We MAY need to handle 1-11 size for extended nid
		var addr [4]uint32
		addr[0] = addr0
		addr123 := addr[1:4]
		err := binary.Read(reader, byteOrder, &addr123)
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

// ToBytes converts the NID64 to a byte slice.
func (nid NID64) ToBytes(byteOrder binary.ByteOrder) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, byteOrder, nid.NIDHeader); err != nil {
		return nil, fmt.Errorf("failed to write NID64: %w", err)
	}
	if err := binary.Write(buf, byteOrder, nid.Addr); err != nil {
		return nil, fmt.Errorf("failed to write NID64: %w", err)
	}
	// NB: port is nonstandard. do not write for NID64
	return buf.Bytes(), nil
}

// ToBytes converts the ExtendedNID to a byte slice.
func (enid ExtendedNID) ToBytes(byteOrder binary.ByteOrder) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, byteOrder, enid.NIDHeader); err != nil {
		return nil, fmt.Errorf("failed to write ExtendedNID: %w", err)
	}
	if err := binary.Write(buf, byteOrder, enid.Addr); err != nil {
		return nil, fmt.Errorf("failed to write ExtendedNID: %w", err)
	}
	// NB: port is nonstandard. do not write for ExtendedNID
	return buf.Bytes(), nil
}

// NetAddr converts the NID64 to a netip.Addr, assuming it's an IPv4 address.
func (nid NID64) NetAddr() netip.Addr {
	var bytes [4]byte
	// NOTE: netip.Addr uses Big endian
	binary.BigEndian.PutUint32(bytes[:4], nid.Addr[0])
	return netip.AddrFrom4(bytes)
}

// NetAddr converts the ExtendedNID to a netip.Addr, assuming it's an IPv6 address.
func (enid ExtendedNID) NetAddr() netip.Addr {
	var bytes []byte
	// NOTE: netip.Addr uses Big endian
	for _, addrValue := range enid.Addr {
		binary.BigEndian.AppendUint32(bytes, addrValue)
	}
	return netip.AddrFrom16([16]byte(bytes))
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
