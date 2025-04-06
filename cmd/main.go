package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/cmd/cli"
	"github.com/theblitlabs/parity-server/internal/core/config"
)

var (
	logMode    string
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "parity-server",
	Short: "Parity Protocol Server",
	Long: `Parity Protocol Server - A server for the Parity Protocol network.
	
This server manages tasks, runners, and rewards in the Parity Protocol network.
Configuration can be loaded from a file specified by the --config flag.
The default config path is ".env".`,
	Example: `  # Run with default config
  parity-server
  
  # Run with custom config
  parity-server --config /path/to/.env`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		switch logMode {
		case "debug", "pretty", "info", "prod", "test":
			gologger.InitWithMode(gologger.LogMode(logMode))
		default:
			gologger.InitWithMode(gologger.LogModePretty)
		}

		if cmd.Flags().Changed("config") {
			config.GetConfigManager().SetConfigPath(configPath)
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
	configPath = ".env"

	if envPath := os.Getenv("PARITY_CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	configManager := config.GetConfigManager()
	configManager.SetConfigPath(configPath)

	// Load config immediately to catch any errors early
	if _, err := configManager.GetConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", configPath, "Path to config file")
	rootCmd.PersistentFlags().StringVar(&logMode, "log", "pretty", "Log mode: debug, pretty, info, prod, test")
	authCmd.Flags().String("private-key", "", "Private key in hex format")
	if err := authCmd.MarkFlagRequired("private-key"); err != nil {
		log.Fatalf("Error marking flag required: %v", err)
	}

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the parity server",
	Example: `  # Start server with default config
  parity-server server
  
  # Start server with custom config
  parity-server server --config /path/to/.env`,
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunServer()
	},
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the server",
	Example: `  # Authenticate with default config
  parity-server auth --private-key YOUR_PRIVATE_KEY
  
  # Authenticate with custom config
  parity-server auth --private-key YOUR_PRIVATE_KEY --config /path/to/.env`,
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunAuth()
	},
}
