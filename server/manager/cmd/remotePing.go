/*
Copyright © 2026 GlimmerFS Project
SPDX-License-Identifier: GPL-2.0
*/
package cmd

import (
	"fmt"
	"log/slog"

	"github.com/glimmerfs/glimmer/wire/lnet"
	"github.com/spf13/cobra"
)

// remotePingCmd represents the remote-ping command
var remotePingCmd = &cobra.Command{
	Use:   "remote-ping",
	Short: "Ping a remote service to check connectivity",
	Long: `Ping a remote service to check connectivity.

This can verify local network connectivity to help with troubleshooting.
Unlike lnetctl ping, this does not require binding to port 1023 and supports
specifying a custom port.
`,
	ValidArgs: []string{"<remote_address>"}, // Placeholder for argument validation
	Args:      cobra.ExactArgs(1),           // Expect exactly one argument (the remote address)
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("remote-ping requires exactly one argument: the remote address")
		}
		NID, err := lnet.ParseNID(args[0])
		if err != nil {
			return err
		}
		if NID.IsAny() {
			return fmt.Errorf("cannot ping 'any' NID")
		}
		slog.Info("pinging remote service", "nid", NID)
		// TODO: implement actual ping logic using LNetClient
		return nil
	},
}

func init() {
	rootCmd.AddCommand(remotePingCmd)
}
