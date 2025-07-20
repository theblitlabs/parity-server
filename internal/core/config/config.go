package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	Server            ServerConfig            `mapstructure:"SERVER"`
	Database          DatabaseConfig          `mapstructure:"DATABASE"`
	BlockchainNetwork BlockchainNetworkConfig `mapstructure:"BLOCKCHAIN_NETWORK"`
	Blockchain        BlockchainConfig        `mapstructure:"BLOCKCHAIN"`
	Scheduler         SchedulerConfig         `mapstructure:"SCHEDULER"`
	Reputation        ReputationConfig        `mapstructure:"REPUTATION"`
	SmartContract     SmartContractConfig     `mapstructure:"SMART_CONTRACT"`
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

type BlockchainConfig struct {
	IPFSEndpoint       string `mapstructure:"IPFS_ENDPOINT"`
	GatewayURL         string `mapstructure:"GATEWAY_URL"`
	CreateStorageDeals bool   `mapstructure:"CREATE_STORAGE_DEALS"`
}

type BlockchainNetworkConfig struct {
	RPC                string `mapstructure:"RPC"`
	ChainID            int64  `mapstructure:"CHAIN_ID"`
	TokenAddress       string `mapstructure:"TOKEN_ADDRESS"`
	StakeWalletAddress string `mapstructure:"STAKE_WALLET_ADDRESS"`
	TokenSymbol        string `mapstructure:"TOKEN_SYMBOL"`
	NetworkName        string `mapstructure:"NETWORK_NAME"`
}

type SchedulerConfig struct {
	Interval int `mapstructure:"INTERVAL"`
}

type ReputationConfig struct {
	MonitoringEnabled  bool `mapstructure:"MONITORING_ENABLED"`
	MonitoringInterval int  `mapstructure:"MONITORING_INTERVAL"`
	AssignmentDuration int  `mapstructure:"ASSIGNMENT_DURATION"`
	MaxAssignments     int  `mapstructure:"MAX_ASSIGNMENTS"`
	SlashingEnabled    bool `mapstructure:"SLASHING_ENABLED"`
	SlashingPercentage int  `mapstructure:"SLASHING_PERCENTAGE"`
	MinimumStake       int  `mapstructure:"MINIMUM_STAKE"`
}

type SmartContractConfig struct {
	ReputationContractAddress string `mapstructure:"REPUTATION_CONTRACT_ADDRESS"`
	ReputationContractABIPath string `mapstructure:"REPUTATION_CONTRACT_ABI_PATH"`
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

	v.SetDefault("BLOCKCHAIN", map[string]interface{}{
		"IPFS_ENDPOINT":        v.GetString("BLOCKCHAIN_IPFS_ENDPOINT"),
		"GATEWAY_URL":          v.GetString("BLOCKCHAIN_GATEWAY_URL"),
		"CREATE_STORAGE_DEALS": v.GetBool("BLOCKCHAIN_CREATE_STORAGE_DEALS"),
	})

	v.SetDefault("BLOCKCHAIN_NETWORK", map[string]interface{}{
		"RPC":                  v.GetString("BLOCKCHAIN_RPC"),
		"CHAIN_ID":             v.GetInt64("BLOCKCHAIN_CHAIN_ID"),
		"TOKEN_ADDRESS":        v.GetString("BLOCKCHAIN_TOKEN_ADDRESS"),
		"STAKE_WALLET_ADDRESS": v.GetString("BLOCKCHAIN_STAKE_WALLET_ADDRESS"),
		"TOKEN_SYMBOL":         v.GetString("BLOCKCHAIN_TOKEN_SYMBOL"),
		"NETWORK_NAME":         v.GetString("BLOCKCHAIN_NETWORK_NAME"),
	})

	v.SetDefault("SCHEDULER", map[string]interface{}{
		"INTERVAL": v.GetInt("SCHEDULER_INTERVAL"),
	})

	v.SetDefault("REPUTATION", map[string]interface{}{
		"MONITORING_ENABLED":  v.GetBool("REPUTATION_MONITORING_ENABLED"),
		"MONITORING_INTERVAL": v.GetInt("REPUTATION_MONITORING_INTERVAL"),
		"ASSIGNMENT_DURATION": v.GetInt("REPUTATION_ASSIGNMENT_DURATION"),
		"MAX_ASSIGNMENTS":     v.GetInt("REPUTATION_MAX_ASSIGNMENTS"),
		"SLASHING_ENABLED":    v.GetBool("REPUTATION_SLASHING_ENABLED"),
		"SLASHING_PERCENTAGE": v.GetInt("REPUTATION_SLASHING_PERCENTAGE"),
		"MINIMUM_STAKE":       v.GetInt("REPUTATION_MINIMUM_STAKE"),
	})

	v.SetDefault("SMART_CONTRACT", map[string]interface{}{
		"REPUTATION_CONTRACT_ADDRESS":  v.GetString("REPUTATION_CONTRACT_ADDRESS"),
		"REPUTATION_CONTRACT_ABI_PATH": v.GetString("REPUTATION_CONTRACT_ABI_PATH"),
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
