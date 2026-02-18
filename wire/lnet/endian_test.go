/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Tests for endian handling utilities in the lnet package.
*/
package lnet

import (
	"encoding/binary"
	"testing"
)

func TestIsLittleEndian(t *testing.T) {
	if !IsLittleEndian(binary.LittleEndian) {
		t.Error("Expected LittleEndian to be little-endian")
	}
	if IsLittleEndian(binary.BigEndian) {
		t.Error("Expected BigEndian to not be little-endian")
	}
}

func TestIsBigEndian(t *testing.T) {
	if IsBigEndian(binary.LittleEndian) {
		t.Error("Expected LittleEndian to not be big-endian")
	}
	if !IsBigEndian(binary.BigEndian) {
		t.Error("Expected BigEndian to be big-endian")
	}
}

func TestGetOppositeByteOrder(t *testing.T) {
	if GetOppositeByteOrder(binary.LittleEndian) != binary.BigEndian {
		t.Error("Expected opposite of LittleEndian to be BigEndian")
	}
	if GetOppositeByteOrder(binary.BigEndian) != binary.LittleEndian {
		t.Error("Expected opposite of BigEndian to be LittleEndian")
	}
}

func TestSwab32(t *testing.T) {
	var tests = []struct {
		input    uint32
		expected uint32
	}{
		{0x01020304, 0x04030201},
		{0xAABBCCDD, 0xDDCCBBAA},
		{0x12345678, 0x78563412},
	}
	for _, test := range tests {
		if Swab32(test.input) != test.expected {
			t.Errorf("Swab32(%#08x) = %#08x; expected %#08x", test.input, Swab32(test.input), test.expected)
		}
	}
}

func TestSwab16(t *testing.T) {
	var tests = []struct {
		input    uint16
		expected uint16
	}{
		{0x0102, 0x0201},
		{0xAABB, 0xBBAA},
		{0x1234, 0x3412},
	}
	for _, test := range tests {
		if Swab16(test.input) != test.expected {
			t.Errorf("Swab16(%#04x) = %#04x; expected %#04x", test.input, Swab16(test.input), test.expected)
		}
	}
}

func TestSwab64(t *testing.T) {
	var tests = []struct {
		input    uint64
		expected uint64
	}{
		{0x0102030405060708, 0x0807060504030201},
		{0x1122334455667788, 0x8877665544332211},
		{0xAABBCCDDEEFF0011, 0x1100FFEEDDCCBBAA},
	}
	for _, test := range tests {
		if Swab64(test.input) != test.expected {
			t.Errorf("Swab64(%#016x) = %#016x; expected %#016x", test.input, Swab64(test.input), test.expected)
		}
	}
}
