/*
Copyright © 2026 GlimmerFS Project

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package cmd

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/spf13/cobra"
)

// lnetDebugCmd represents the lnetDebug command
var lnetDebugCmd = &cobra.Command{
	Use:   "lnet-debug",
	Short: "Debug an LNet TCP Connection from a Lustre Client",
	Long: `This performs initial communication to help troubleshoot lnet implementation.

	Only TCP is supported.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		listenConf := net.ListenConfig{}
		ctx := cmd.Context()
		listener, err := listenConf.Listen(ctx, "tcp", fmt.Sprintf(":%d", Port))
		if err != nil {
			return err
		}
		defer listener.Close()
		slog.Info("lnet-debug listening", "port", Port)

		context.AfterFunc(ctx, func() {
			slog.Debug("lnet-debug listener shutting down")
			listener.Close()
		})

		for {
			slog.Debug("lnet-debug waiting for connection")
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					// Context was cancelled, exit gracefully
					return nil
				}
				slog.Error("Accept Error", "error", err)
				return err
			}
			go handleConnection(conn)
		}
	},
}

func init() {
	rootCmd.AddCommand(lnetDebugCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// lnetDebugCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// lnetDebugCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Constants from lustre_idl.h and socklnd.h
const (
	LNetProtoMagic         = 0x45726963 // General proto switcher
	LNetProtoTCPMagic      = 0xeebc0ded // The Magic Number
	LNetProtoAcceptorMagic = 0xacce7100 // The "Handshake" Magic
	Port                   = 988        // Default LNet Port
)

const (
	KSockProtoV3 = 3
)

type HexUInt64 uint64
type HexUInt32 uint32

func (h HexUInt64) String() string {
	return fmt.Sprintf("0x%016x", uint64(h))
}

func (h HexUInt32) String() string {
	return fmt.Sprintf("0x%08x", uint32(h))
}

// Matches 'struct lnet_acceptor_connreq' in lustre_idl.h
type AcceptorConnReq struct {
	// Magic is read separately first
	Version HexUInt32 // Protocol version
	Nid     HexUInt64 // The Client's NID
}

type ProtoResp struct {
	Magic   HexUInt32 // LNetProtoAcceptorMagic
	Version HexUInt32 // Echoed Version
	Nid     HexUInt64 // The Client's NID
}

// HelloMsg is the first thing exchanged on a TCP connection
// Matches 'struct ksock_hello_msg' in socklnd.h
type HelloMsg struct {
	// LNetProtoMagic is read first
	// ProtoV3 is read next
	SrcNID         HexUInt64 // Sender NID
	DstNID         HexUInt64 // Receiver NID
	SrcPID         HexUInt32 // Sender PID
	DstPID         HexUInt32 // Receiver PID
	SrcIncarnation HexUInt64 // Sender Incarnation
	DstIncarnation HexUInt64 // Receiver Incarnation
	ConnType       HexUInt32 // Connection Type (SOCKLND_CONN_*)
	NIPs           HexUInt32 // Always 0
	// IPs uint32[] Unsupported (zero length array)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()

	// 1. Read the Magic (First 4 bytes)
	var magic HexUInt32
	if err := binary.Read(conn, binary.LittleEndian, &magic); err != nil {
		slog.Error("[%s] Read Error: %v\n", remoteAddr, err)
		return
	}
	if magic != LNetProtoAcceptorMagic {
		slog.Error("Invalid Magic", "addr", remoteAddr, "magic", magic)
		return
	}
	err := handleAcceptor(conn, remoteAddr)
	if err != nil {
		slog.Error("Acceptor Handling Error", "addr", remoteAddr, "error", err)
	}

	// 2. Read based on Protocol
	if err := binary.Read(conn, binary.LittleEndian, &magic); err != nil {
		slog.Error("[%s] Read Error: %v\n", remoteAddr, err)
		return
	}
	switch magic {
	case LNetProtoMagic:
		slog.Info("Magic: Generic", "addr", remoteAddr)
	case LNetProtoTCPMagic:
		slog.Info("Magic: TCP/LND", "addr", remoteAddr)
	default:
		slog.Error("UNKNOWN MAGIC", "addr", remoteAddr, "magic", magic)
	}

	// 3. Read the Command (Next 4 bytes)
	var cmd HexUInt32
	if err := binary.Read(conn, binary.LittleEndian, &cmd); err != nil {
		slog.Error("[%s] Read Error: %v\n", remoteAddr, err)
		return
	}
	switch cmd {
	case KSockProtoV3:
		slog.Info("Received HELLO V3 Command", "addr", remoteAddr)
		if err := handleHelloV3(conn, remoteAddr); err != nil {
			slog.Error("Hello Handling Error", "addr", remoteAddr, "error", err)
		}
	default:
		slog.Error("UNKNOWN COMMAND", "addr", remoteAddr, "cmd", cmd)
	}
}

func handleAcceptor(conn net.Conn, remoteAddr string) error {
	// We already read the magic, so we read the REST of the struct
	var req AcceptorConnReq
	if err := binary.Read(conn, binary.LittleEndian, &req); err != nil {
		return err
	}
	if req.Version != 1 {
		return errors.New("unsupported LNet protocol version")
	}

	slog.Info("Acceptor Connection Request", "addr", remoteAddr,
		"client_nid", req.Nid,
		"version", req.Version,
	)
	return nil
}

func handleHelloV3(conn net.Conn, remoteAddr string) error {
	var hello HelloMsg
	if err := binary.Read(conn, binary.LittleEndian, &hello); err != nil {
		fmt.Printf("[%s] Failed to read HelloMsg: %v\n", remoteAddr, err)
		return err
	}
	slog.Info("Got HELLO V3", "addr", remoteAddr, "hello", hello)
	if hello.NIPs != 0 {
		slog.Error("Non-zero NIPs not supported", "addr", remoteAddr, "nips", hello.NIPs)
		return errors.ErrUnsupported
	}
	return nil
}
