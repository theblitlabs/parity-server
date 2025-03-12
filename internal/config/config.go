package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Ethereum EthereumConfig `mapstructure:"ethereum"`
	Runner   RunnerConfig   `mapstructure:"runner"`
	IPFS     IPFSConfig     `mapstructure:"ipfs"`
}

type ServerConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Endpoint string `mapstructure:"endpoint"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	URL             string        `mapstructure:"url"`
}

type EthereumConfig struct {
	RPC                string `mapstructure:"rpc"`
	ChainID            int64  `mapstructure:"chain_id"`
	TokenAddress       string `mapstructure:"token_address"`
	StakeWalletAddress string `mapstructure:"stake_wallet_address"`
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

type RunnerConfig struct {
	ServerURL    string           `mapstructure:"server_url"`
	WebsocketURL string           `mapstructure:"websocket_url"`
	WebhookPort  int              `mapstructure:"webhook_port"`
	APIPrefix    string           `mapstructure:"api_prefix"`
	Docker       DockerConfig     `mapstructure:"docker"`
	IPFS         IPFSRunnerConfig `mapstructure:"ipfs"`
}

type DockerConfig struct {
	MemoryLimit string        `mapstructure:"memory_limit"`
	CPULimit    string        `mapstructure:"cpu_limit"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type IPFSConfig struct {
	APIURL string `mapstructure:"api_url"`
}

type IPFSRunnerConfig struct {
	Image       string `mapstructure:"image"`
	APIPort     int    `mapstructure:"api_port"`
	GatewayPort int    `mapstructure:"gateway_port"`
	SwarmPort   int    `mapstructure:"swarm_port"`
	DataDir     string `mapstructure:"data_dir"`
}
