package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"SERVER"`
	Database  DatabaseConfig  `mapstructure:"DATABASE"`
	Ethereum  EthereumConfig  `mapstructure:"ETHEREUM"`
	AWS       AWSConfig       `mapstructure:"AWS"`
	Scheduler SchedulerConfig `mapstructure:"SCHEDULER"`
}

type ServerConfig struct {
	Host     string `mapstructure:"HOST"`
	Port     string `mapstructure:"PORT"`
	Endpoint string `mapstructure:"ENDPOINT"`
}

type DatabaseConfig struct {
	Username      string `mapstructure:"USERNAME"`
	Password      string `mapstructure:"PASSWORD"`
	Host          string `mapstructure:"HOST"`
	Port          string `mapstructure:"PORT"`
	Database_name string `mapstructure:"DATABASE_NAME"`
}

type AWSConfig struct {
	Region     string `mapstructure:"REGION"`
	BucketName string `mapstructure:"BUCKET_NAME"`
}

type EthereumConfig struct {
	RPC                string `mapstructure:"RPC"`
	ChainID            int64  `mapstructure:"CHAIN_ID"`
	TokenAddress       string `mapstructure:"TOKEN_ADDRESS"`
	StakeWalletAddress string `mapstructure:"STAKE_WALLET_ADDRESS"`
}

type SchedulerConfig struct {
	Interval int `mapstructure:"INTERVAL"`
}

type ConfigManager struct {
	config     *Config
	configPath string
	mutex      sync.RWMutex
}

var (
	instance *ConfigManager
	once     sync.Once
)

func (dc *DatabaseConfig) GetConnectionURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		dc.Username,
		dc.Password,
		dc.Host,
		dc.Port,
		dc.Database_name,
	)
}

func GetConfigManager() *ConfigManager {
	once.Do(func() {
		instance = &ConfigManager{
			configPath: ".env",
		}
	})
	return instance
}

func (cm *ConfigManager) SetConfigPath(path string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.configPath = path
	cm.config = nil
}

func (cm *ConfigManager) GetConfig() (*Config, error) {
	cm.mutex.RLock()
	if cm.config != nil {
		defer cm.mutex.RUnlock()
		return cm.config, nil
	}
	cm.mutex.RUnlock()

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.config != nil {
		return cm.config, nil
	}

	var err error
	cm.config, err = loadConfigFile(cm.configPath)
	return cm.config, err
}

func (cm *ConfigManager) ReloadConfig() (*Config, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	var err error
	cm.config, err = loadConfigFile(cm.configPath)
	return cm.config, err
}

func loadConfigFile(path string) (*Config, error) {
	v := viper.New()

	v.SetEnvPrefix("")
	v.AutomaticEnv()

	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config

	// Bind environment variables
	envVars := []string{
		"SERVER_HOST",
		"SERVER_PORT",
		"SERVER_ENDPOINT",
		"DATABASE_USERNAME",
		"DATABASE_PASSWORD",
		"DATABASE_HOST",
		"DATABASE_PORT",
		"DATABASE_DATABASE_NAME",
		"AWS_REGION",
		"AWS_BUCKET_NAME",
		"ETHEREUM_RPC",
		"ETHEREUM_CHAIN_ID",
		"ETHEREUM_TOKEN_ADDRESS",
		"ETHEREUM_STAKE_WALLET_ADDRESS",
		"SCHEDULER_INTERVAL",
	}

	for _, env := range envVars {
		if err := v.BindEnv(env); err != nil {
			return nil, fmt.Errorf("failed to bind env var %s: %w", env, err)
		}
	}

	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	return &config, nil
}

func (cm *ConfigManager) GetConfigPath() string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.configPath
}
