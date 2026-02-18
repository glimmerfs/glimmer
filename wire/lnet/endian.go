/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Network endian handling and related utilities.
*/
package lnet

import (
	"encoding/binary"
)

type rwByteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

// we use native-endian on our side (receiver). on the remote side, we need to detect and do flipping if needed.
// NOTE: this is going to be LittleEndian in almost all cases
// We try to not set this to NativeEndian to make it easier to swap
var DEFAULT_BYTE_ORDER rwByteOrder

// IsLittleEndian reports whether the given byte order is little-endian.
// This works for NativeEndian as well
func IsLittleEndian(byteOrder binary.ByteOrder) bool {
	switch byteOrder {
	case binary.LittleEndian:
		return true
	case binary.BigEndian:
		return false
	}
	var nativeBuff [4]byte
	var leBuff [4]byte
	byteOrder.PutUint32(nativeBuff[:], 0x01020304)
	binary.LittleEndian.PutUint32(leBuff[:], 0x01020304)
	return nativeBuff == leBuff
}

// IsLittleEndian reports whether the given byte order is big-endian.
func IsBigEndian(byteOrder binary.ByteOrder) bool {
	return !IsLittleEndian(byteOrder)
}

// GetOppositeByteOrder returns the opposite of the given byte order (little vs big).
func GetOppositeByteOrder(byteOrder binary.ByteOrder) binary.ByteOrder {
	if IsLittleEndian(byteOrder) {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// Swab32 reverses the byte order of a 32-bit unsigned integer.
// AABBCCDD -> DDCCBBAA
func Swab32(x uint32) uint32 {
	return ((x >> 24) & 0xFF) | ((x >> 8) & 0xFF00) | ((x << 8) & 0xFF0000) | ((x << 24) & 0xFF000000)
}

// Swab16 reverses the byte order of a 16-bit unsigned integer.
// AABB -> BBAA
func Swab16(x uint16) uint16 {
	return (x >> 8) | (x << 8)
}

// Swab64 reverses the byte order of a 64-bit unsigned integer.
// 1122334455667788 -> 8877665544332211
func Swab64(x uint64) uint64 {
	return ((x >> 56) & 0xFF) | ((x >> 40) & 0xFF00) | ((x >> 24) & 0xFF0000) | ((x >> 8) & 0xFF000000) |
		((x << 8) & 0xFF00000000) | ((x << 24) & 0xFF0000000000) | ((x << 40) & 0xFF000000000000) | ((x << 56) & 0xFF00000000000000)
}

func init() {
	if IsLittleEndian(binary.NativeEndian) {
		DEFAULT_BYTE_ORDER = binary.LittleEndian
	} else {
		DEFAULT_BYTE_ORDER = binary.BigEndian
	}
}
