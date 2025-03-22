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

var logMode string
var configPath string

var rootCmd = &cobra.Command{
	Use:   "parity-server",
	Short: "Parity Protocol Server",
	Long: `Parity Protocol Server - A server for the Parity Protocol network.
	
This server manages tasks, runners, and rewards in the Parity Protocol network.
Configuration can be loaded from a file specified by the --config flag.
The default config path is "config/config.yaml".`,
	Example: `  # Run with default config
  parity-server
  
  # Run with custom config
  parity-server --config /path/to/config.yaml`,
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
	// Default config path
	configPath = "config/config.yaml"
	
	// Check for environment variable override
	if envPath := os.Getenv("PARITY_CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}
	
	// Pre-initialize config
	configManager := config.GetConfigManager()
	configManager.SetConfigPath(configPath)
	
	// Add global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", configPath, "Path to config file")
	rootCmd.PersistentFlags().StringVar(&logMode, "log", "pretty", "Log mode: debug, pretty, info, prod, test")
	authCmd.Flags().String("private-key", "", "Private key in hex format")
	if err := authCmd.MarkFlagRequired("private-key"); err != nil {
		log.Fatalf("Error marking flag required: %v", err)
	}

	// Add push-task command
	pushTaskCmd.Flags().String("task-id", "", "ID of the task to push")
	pushTaskCmd.Flags().String("runner-id", "", "ID (device ID) of the runner to push the task to")
	if err := pushTaskCmd.MarkFlagRequired("task-id"); err != nil {
		log.Fatalf("Error marking flag required: %v", err)
	}
	if err := pushTaskCmd.MarkFlagRequired("runner-id"); err != nil {
		log.Fatalf("Error marking flag required: %v", err)
	}

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(pushTaskCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the parity server",
	Example: `  # Start server with default config
  parity-server server
  
  # Start server with custom config
  parity-server server --config /path/to/config.yaml`,
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
  parity-server auth --private-key YOUR_PRIVATE_KEY --config /path/to/config.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunAuth()
	},
}

var pushTaskCmd = &cobra.Command{
	Use:   "push-task",
	Short: "Push a task to a specific runner",
	Example: `  # Push task using default config
  parity-server push-task --task-id TASK_ID --runner-id RUNNER_ID
  
  # Push task using custom config
  parity-server push-task --task-id TASK_ID --runner-id RUNNER_ID --config /path/to/config.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		taskID, _ := cmd.Flags().GetString("task-id")
		runnerID, _ := cmd.Flags().GetString("runner-id")

		cfg, err := config.GetConfigManager().GetConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}

		if err := cli.PushTaskToRunner(taskID, runnerID, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to push task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully pushed task %s to runner %s\n", taskID, runnerID)
	},
}
