package common

import (
	"os"
	"strconv"
)

type Config struct {
	Port            int
	DatabaseDSN     string
	OpenClawDir     string
	DefaultBotID    string
	FeishuAppID     string
	FeishuAppSecret string
}

func LoadConfig() *Config {
	port, _ := strconv.Atoi(getEnv("PORT", "8090"))
	return &Config{
		Port:            port,
		DatabaseDSN:     getEnv("DATABASE_DSN", "host=localhost user=collab password=collab dbname=collab port=5432 sslmode=disable"),
		OpenClawDir:     getEnv("OPENCLAW_DIR", os.ExpandEnv("$HOME/.openclaw")),
		DefaultBotID:    getEnv("DEFAULT_BOT_ID", ""),
		FeishuAppID:     getEnv("FEISHU_APP_ID", ""),
		FeishuAppSecret: getEnv("FEISHU_APP_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
