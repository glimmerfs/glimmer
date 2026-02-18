//go:build 386 || amd64 || amd64p32 || alpha || arm || arm64 || loong64 || mipsle || mips64le || mips64p32le || nios2 || ppc64le || riscv || riscv64 || sh || wasm

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
	if !IsLittleEndian(DEFAULT_BYTE_ORDER) {
		t.Error("Expected DEFAULT_BYTE_ORDER to be little-endian on this platform")
	}
	if IsBigEndian(DEFAULT_BYTE_ORDER) {
		t.Error("Expected DEFAULT_BYTE_ORDER to not be big-endian on this platform")
	}
	if GetOppositeByteOrder(DEFAULT_BYTE_ORDER) != binary.BigEndian {
		t.Error("Expected opposite of DEFAULT_BYTE_ORDER to be BigEndian")
	}
}
