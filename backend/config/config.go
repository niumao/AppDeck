package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Data     DataConfig
	WeChat   WeChatConfig
	JWT      JWTConfig
}

type WeChatConfig struct {
	AppID          string
	AppSecret      string
	Token          string
	EncodingAESKey string
	MchID          string
	MchAPIKey      string
	MchCertPath    string
	MchKeyPath     string
	PayNotifyURL   string
	// Official Account config (may differ from OAuth AppID)
	OAAppID       string
	OAAppSecret   string
	OAFollowURL   string // QR code image URL for following the Official Account (displayed in H5 follow gate)
}

type JWTConfig struct {
	Secret      string
	ExpiryHours int
}

type ServerConfig struct {
	Port    string
	Host    string
	BaseURL string // e.g. "http://xiongmaovr.cn" or "http://xiongmaovr.cn/test"
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	Charset  string
}

type DataConfig struct {
	IconsDir   string
	BigPicsDir string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	return &Config{
		Server: ServerConfig{
			Port:    getEnv("SERVER_PORT", "8080"),
			Host:    getEnv("SERVER_HOST", "0.0.0.0"),
			BaseURL: getEnv("APP_BASE_URL", "http://xiongmaovr.cn"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "appdeck"),
			Password: getEnv("DB_PASSWORD", "appdeck"),
			DBName:   getEnv("DB_NAME", "appdeck"),
			Charset:  getEnv("DB_CHARSET", "utf8mb4"),
		},
		Data: DataConfig{
			IconsDir:   getEnv("DATA_ICONS_DIR", "./data/icons"),
			BigPicsDir: getEnv("DATA_BIGPICS_DIR", "./data/bigpics"),
		},
		WeChat: WeChatConfig{
			AppID:          getEnv("WECHAT_APP_ID", ""),
			AppSecret:      getEnv("WECHAT_APP_SECRET", ""),
			Token:          getEnv("WECHAT_TOKEN", ""),
			EncodingAESKey: getEnv("WECHAT_ENCODING_AES_KEY", ""),
			MchID:          getEnv("WECHAT_MCH_ID", ""),
			MchAPIKey:      getEnv("WECHAT_MCH_API_KEY", ""),
			MchCertPath:    getEnv("WECHAT_MCH_CERT_PATH", ""),
			MchKeyPath:     getEnv("WECHAT_MCH_KEY_PATH", ""),
			PayNotifyURL:   getEnv("WECHAT_PAY_NOTIFY_URL", ""),
			// Official Account (fallback to main AppID if not set separately)
			OAAppID:       getEnv("WECHAT_OA_APPID", getEnv("WECHAT_APP_ID", "")),
			OAAppSecret:   getEnv("WECHAT_OA_APPSECRET", getEnv("WECHAT_APP_SECRET", "")),
			OAFollowURL:   getEnv("WECHAT_OA_FOLLOW_URL", ""),
		},
		JWT: JWTConfig{
			Secret:      getEnv("JWT_SECRET", "appdeck-jwt-secret-key"),
			ExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 168),
		},
	}
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
