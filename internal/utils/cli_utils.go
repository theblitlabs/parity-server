package utils

import (
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type CommandConfig struct {
	Use     string
	Short   string
	Long    string
	Example string

	RunFunc func(cmd *cobra.Command, args []string) error

	Flags map[string]Flag
}

type Flag struct {
	Type        FlagType
	Shorthand   string
	Description string
	Required    bool

	DefaultString  string
	DefaultInt     int
	DefaultFloat64 float64
	DefaultBool    bool
}

type FlagType int

const (
	FlagTypeString FlagType = iota
	FlagTypeInt
	FlagTypeFloat64
	FlagTypeBool
)

func CreateCommand(config CommandConfig, log zerolog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:     config.Use,
		Short:   config.Short,
		Long:    config.Long,
		Example: config.Example,
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.RunFunc != nil {
				return config.RunFunc(cmd, args)
			}
			return nil
		},
	}

	for name, flag := range config.Flags {
		switch flag.Type {
		case FlagTypeString:
			cmd.Flags().StringP(name, flag.Shorthand, flag.DefaultString, flag.Description)
		case FlagTypeInt:
			cmd.Flags().IntP(name, flag.Shorthand, flag.DefaultInt, flag.Description)
		case FlagTypeFloat64:
			cmd.Flags().Float64P(name, flag.Shorthand, flag.DefaultFloat64, flag.Description)
		case FlagTypeBool:
			cmd.Flags().BoolP(name, flag.Shorthand, flag.DefaultBool, flag.Description)
		}

		if flag.Required {
			if err := cmd.MarkFlagRequired(name); err != nil {
				log.Error().Err(err).Str("flag", name).Msg("Failed to mark flag as required")
			}
		}
	}

	return cmd
}

func ExecuteCommand(cmd *cobra.Command, log zerolog.Logger) error {
	if err := cmd.Execute(); err != nil {
		log.Error().Err(err).Msg("Command execution failed")
		return err
	}
	return nil
}
