package config

import (
	"fmt"
	"strings"
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
	Username     string `mapstructure:"USERNAME"`
	Password     string `mapstructure:"PASSWORD"`
	Host         string `mapstructure:"HOST"`
	Port         string `mapstructure:"PORT"`
	DatabaseName string `mapstructure:"DATABASE_NAME"`
}

type AWSConfig struct {
	Region          string `mapstructure:"REGION"`
	BucketName      string `mapstructure:"BUCKET_NAME"`
	AccessKeyID     string `mapstructure:"ACCESS_KEY_ID"`
	SecretAccessKey string `mapstructure:"SECRET_ACCESS_KEY"`
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
		dc.DatabaseName,
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

	v.SetConfigFile(path)
	v.SetEnvPrefix("")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	v.SetDefault("SERVER", map[string]interface{}{
		"HOST":     v.GetString("SERVER_HOST"),
		"PORT":     v.GetString("SERVER_PORT"),
		"ENDPOINT": v.GetString("SERVER_ENDPOINT"),
	})

	v.SetDefault("DATABASE", map[string]interface{}{
		"USERNAME":      v.GetString("DATABASE_USERNAME"),
		"PASSWORD":      v.GetString("DATABASE_PASSWORD"),
		"HOST":          v.GetString("DATABASE_HOST"),
		"PORT":          v.GetString("DATABASE_PORT"),
		"DATABASE_NAME": v.GetString("DATABASE_DATABASE_NAME"),
	})

	v.SetDefault("AWS", map[string]interface{}{
		"REGION":            v.GetString("AWS_REGION"),
		"BUCKET_NAME":       v.GetString("AWS_BUCKET_NAME"),
		"ACCESS_KEY_ID":     v.GetString("AWS_ACCESS_KEY_ID"),
		"SECRET_ACCESS_KEY": v.GetString("AWS_SECRET_ACCESS_KEY"),
	})

	v.SetDefault("ETHEREUM", map[string]interface{}{
		"RPC":                  v.GetString("ETHEREUM_RPC"),
		"CHAIN_ID":             v.GetInt64("ETHEREUM_CHAIN_ID"),
		"TOKEN_ADDRESS":        v.GetString("ETHEREUM_TOKEN_ADDRESS"),
		"STAKE_WALLET_ADDRESS": v.GetString("ETHEREUM_STAKE_WALLET_ADDRESS"),
	})

	v.SetDefault("SCHEDULER", map[string]interface{}{
		"INTERVAL": v.GetInt("SCHEDULER_INTERVAL"),
	})

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	if config.Database.Username == "" || config.Database.Password == "" ||
		config.Database.Host == "" || config.Database.Port == "" ||
		config.Database.DatabaseName == "" {
		return nil, fmt.Errorf("missing required database configuration")
	}

	return &config, nil
}

func (cm *ConfigManager) GetConfigPath() string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.configPath
}
