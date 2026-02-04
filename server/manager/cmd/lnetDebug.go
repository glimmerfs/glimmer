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
	"encoding/binary"
	"fmt"
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
	Run: func(cmd *cobra.Command, args []string) {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
		if err != nil {
			panic(err)
		}
		defer listener.Close()
		fmt.Printf("Glimmer listening on %d...\n", Port)

		for {
			conn, err := listener.Accept()
			if err == nil {
				go handleConnection(conn)
			}
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
	LNetProtoTCPMagic      = 0xeebc0ded // The Magic Number
	LNetProtoAcceptorMagic = 0xacce7100 // The "Handshake" Magic
	Port                   = 988        // Default LNet Port
)

// Matches 'struct lnet_acceptor_connreq' in lustre_idl.h
type AcceptorConnReq struct {
	// Magic is read separately first
	Version uint32 // Protocol version
	Nid     uint64 // The Client's NID
}

// HelloMsg is the first thing exchanged on a TCP connection
// Matches 'struct ksock_hello_msg' in socklnd.h
type HelloMsg struct {
	TxNID        uint64 // Sender NID
	Incarnation  uint64 // Connection instance ID
	Type         uint32 // Message type (HELLO vs NOOP)
	IsBodyOneReg uint32 // Capability flag
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()

	// 1. Read the Magic (First 4 bytes)
	var magic uint32
	if err := binary.Read(conn, binary.LittleEndian, &magic); err != nil {
		fmt.Printf("[%s] Read Error: %v\n", remoteAddr, err)
		return
	}

	// 2. Switch based on Magic
	switch magic {
	case LNetProtoAcceptorMagic:
		fmt.Printf("[%s] Magic: ACCEPTOR (0x%x)\n", remoteAddr, magic)
		handleAcceptor(conn, remoteAddr)
	case LNetProtoTCPMagic:
		fmt.Printf("[%s] Magic: TCP/LND (0x%x)\n", remoteAddr, magic)
		handleHello(conn, remoteAddr)
	default:
		panic(fmt.Sprintf("[%s] UNKNOWN MAGIC: 0x%x\n", remoteAddr, magic))
	}
}

func handleAcceptor(conn net.Conn, remoteAddr string) {
	// We already read the magic, so we read the REST of the struct
	var req AcceptorConnReq
	if err := binary.Read(conn, binary.LittleEndian, &req); err != nil {
		fmt.Printf("[%s] Failed to read AcceptorReq: %v\n", remoteAddr, err)
		return
	}

	fmt.Printf("   >>> Client NID: %d (Version: %d)\n", req.Nid, req.Version)
	fmt.Println("   >>> NOTE: To continue, we must send a response back.")
	binary.Write(conn, binary.LittleEndian, LNetProtoAcceptorMagic)
}

func handleHello(conn net.Conn, remoteAddr string) {
	var hello HelloMsg
	if err := binary.Read(conn, binary.LittleEndian, &hello); err != nil {
		fmt.Printf("[%s] Failed to read HelloMsg: %v\n", remoteAddr, err)
		return
	}
	fmt.Printf("   >>> Hello TxNID: %d\n", hello.TxNID)
}
