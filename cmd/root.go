package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "trustctl",
	Short: "trustctl - certificate automation agent",
	Long:  "trustctl automates certificate issuance and renewal for Let's Encrypt and enterprise CAs.",
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func init() {
	// global persistent flags could be added here
	if os.Geteuid() != 0 {
		// Warn but allow non-root for development; production expects root-owned install
		fmt.Fprintln(os.Stderr, "warning: running as non-root; production expects root ownership of /opt/trustctl")
	}
}
