package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram   TelegramConfig   `yaml:"telegram"`
	Dujiao     DujiaoConfig     `yaml:"dujiao"`
	StockAlert StockAlertConfig `yaml:"stock_alert"`
}

type TelegramConfig struct {
	BotToken     string  `yaml:"bot_token"`
	AllowedUsers []int64 `yaml:"allowed_users"`
}

type DujiaoConfig struct {
	BaseURL            string        `yaml:"base_url"`
	AdminUsername      string        `yaml:"admin_username"`
	AdminPassword      string        `yaml:"admin_password"`
	JWTRefreshInterval time.Duration `yaml:"jwt_refresh_interval"`
}

type StockAlertConfig struct {
	CheckInterval time.Duration `yaml:"check_interval"`
	Threshold     int           `yaml:"threshold"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.applyEnvOverrides()
	cfg.applyDefaults()

	return &cfg, nil
}

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("BOT_TOKEN"); v != "" {
		c.Telegram.BotToken = v
	}
	if v := os.Getenv("DUJIAO_USERNAME"); v != "" {
		c.Dujiao.AdminUsername = v
	}
	if v := os.Getenv("DUJIAO_PASSWORD"); v != "" {
		c.Dujiao.AdminPassword = v
	}
	if v := os.Getenv("DUJIAO_BASE_URL"); v != "" {
		c.Dujiao.BaseURL = v
	}
}

func (c *Config) applyDefaults() {
	if c.Dujiao.JWTRefreshInterval == 0 {
		c.Dujiao.JWTRefreshInterval = 30 * time.Minute
	}
	if c.StockAlert.CheckInterval == 0 {
		c.StockAlert.CheckInterval = 5 * time.Minute
	}
	if c.StockAlert.Threshold == 0 {
		c.StockAlert.Threshold = 10
	}
}

func (c *Config) IsAllowedUser(userID int64) bool {
	for _, id := range c.Telegram.AllowedUsers {
		if id == userID {
			return true
		}
	}
	return false
}
