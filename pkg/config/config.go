package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Queue    QueueConfig
	Crypto   CryptoConfig
	Auth     AuthConfig
	Worker   WorkerConfig
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"` // postgres only for now
	DSN    string `mapstructure:"dsn"`
}

type QueueConfig struct {
	Workers int `mapstructure:"workers"`
}

type CryptoConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"`
	// When AFLOW_AUTH_DISABLED=true, all auth checks are skipped (dev only).
}

type WorkerConfig struct {
	MetricsPort int `mapstructure:"metrics_port"`
}

func Load() (*Config, error) {
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("database.driver", "postgres")
	viper.SetDefault("database.dsn", "postgres://localhost/aflow?sslmode=disable")
	viper.SetDefault("queue.workers", 4)
	viper.SetDefault("worker.metrics_port", 9091)

	viper.SetConfigName("aflow")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.aflow")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("AFLOW")

	_ = viper.BindEnv("crypto.encryption_key", "APP_ENCRYPTION_KEY", "AFLOW_CRYPTO_ENCRYPTION_KEY")
	_ = viper.BindEnv("auth.jwt_secret", "AFLOW_JWT_SECRET")

	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
