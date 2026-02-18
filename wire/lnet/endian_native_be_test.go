//go:build armbe || arm64be || m68k || mips || mips64 || mips64p32 || ppc || ppc64 || s390 || s390x || shbe || sparc || sparc64

/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Tests for when the native byte order is little-endian, which is the most common case on modern hardware.
*/
package lnet

import (
	"encoding/binary"
	"testing"
)

func TestNativeEndian(t *testing.T) {
	if !IsBigEndian(DEFAULT_BYTE_ORDER) {
		t.Error("Expected DEFAULT_BYTE_ORDER to be big-endian on this platform")
	}
	if IsLittleEndian(DEFAULT_BYTE_ORDER) {
		t.Error("Expected DEFAULT_BYTE_ORDER to not be little-endian on this platform")
	}
	if GetOppositeByteOrder(DEFAULT_BYTE_ORDER) != binary.LittleEndian {
		t.Error("Expected opposite of DEFAULT_BYTE_ORDER to be LittleEndian")
	}
}
