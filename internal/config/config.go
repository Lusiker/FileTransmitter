package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Device   DeviceConfig   `mapstructure:"device"`
	Transfer TransferConfig `mapstructure:"transfer"`
	Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
	HTTPPort     int `mapstructure:"http_port"`
	DiscoveryPort int `mapstructure:"discovery_port"`
}

type DeviceConfig struct {
	ID   string `mapstructure:"id"`
	Name string `mapstructure:"name"`
	Role string `mapstructure:"role"`
}

type TransferConfig struct {
	ChunkSize     int64  `mapstructure:"chunk_size"`
	MaxConcurrent int    `mapstructure:"max_concurrent"`
	TempDir       string `mapstructure:"temp_dir"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

var GlobalConfig *Config

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values
	v.SetDefault("server.http_port", 8080)
	v.SetDefault("server.discovery_port", 37021)
	v.SetDefault("device.role", "sender")
	v.SetDefault("transfer.chunk_size", 1048576)
	v.SetDefault("transfer.max_concurrent", 3)
	v.SetDefault("transfer.temp_dir", "./tmp")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

	// Read config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found, use defaults
	}

	// Allow environment variable overrides
	v.SetEnvPrefix("FT")
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Generate device ID if not set
	if cfg.Device.ID == "" {
		cfg.Device.ID = generateDeviceID()
	}

	// Generate device name if not set
	if cfg.Device.Name == "" {
		cfg.Device.Name = getHostname()
	}

	GlobalConfig = &cfg
	return &cfg, nil
}

func generateDeviceID() string {
	// Simple device ID generation using hostname and timestamp
	return fmt.Sprintf("%s-%d", getHostname(), os.Getpid())
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}