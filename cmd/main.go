package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/cmd/cli"
)

var logMode string

var rootCmd = &cobra.Command{
	Use:   "parity-server",
	Short: "Parity Protocol Server",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		switch logMode {
		case "debug", "pretty", "info", "prod", "test":
			gologger.InitWithMode(gologger.LogMode(logMode))
		default:
			gologger.InitWithMode(gologger.LogModePretty)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunServer()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func ExecuteServer() error {
	serverCmd.Run(serverCmd, []string{})
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logMode, "log", "pretty", "Log mode: debug, pretty, info, prod, test")
	authCmd.Flags().String("private-key", "", "Private key in hex format")
	if err := authCmd.MarkFlagRequired("private-key"); err != nil {

	}

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the parity server",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunServer()
	},
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the server",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunAuth()
	},
}
