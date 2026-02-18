/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0

Network protocols and types.
*/
package lnet

import (
	"testing"
)

func TestProtocolAcceptorMagic(t *testing.T) {
	if uint32(PROTO_MAGIC_ACCEPTOR_REV) != Swab32(uint32(PROTO_MAGIC_ACCEPTOR)) {
		t.Errorf("PROTO_MAGIC_ACCEPTOR_REV (0x%08x) is not the byte-swapped version of PROTO_MAGIC_ACCEPTOR (0x%08x)", PROTO_MAGIC_ACCEPTOR_REV, PROTO_MAGIC_ACCEPTOR)
	}
}
