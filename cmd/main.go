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

var pushTaskCmd = &cobra.Command{
	Use:   "push-task",
	Short: "Push a task to a specific runner",
	Run: func(cmd *cobra.Command, args []string) {
		taskID, _ := cmd.Flags().GetString("task-id")
		runnerID, _ := cmd.Flags().GetString("runner-id")

		cfg, err := config.LoadConfig("config/config.yaml")
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
