package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Ethereum  EthereumConfig  `mapstructure:"ethereum"`
	AWS       AWSConfig       `mapstructure:"aws"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
}

type ServerConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Endpoint string `mapstructure:"endpoint"`
}

type DatabaseConfig struct {
	URL string `mapstructure:"url"`
}

type AWSConfig struct {
	Region     string `mapstructure:"region"`
	BucketName string `mapstructure:"bucket_name"`
}

type EthereumConfig struct {
	RPC                string `mapstructure:"rpc"`
	ChainID            int64  `mapstructure:"chain_id"`
	TokenAddress       string `mapstructure:"token_address"`
	StakeWalletAddress string `mapstructure:"stake_wallet_address"`
}

type SchedulerConfig struct {
	Interval int `mapstructure:"interval"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
