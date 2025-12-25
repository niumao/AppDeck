package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Data     DataConfig
}

type ServerConfig struct {
	Port string
	Host string
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
	IconsDir     string
	BigPicsDir   string
	ClientDBPath string
	ResourcesZip string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
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
			IconsDir:     getEnv("DATA_ICONS_DIR", "./data/icons"),
			BigPicsDir:   getEnv("DATA_BIGPICS_DIR", "./data/bigpics"),
			ClientDBPath: getEnv("DATA_CLIENT_DB", "./data/client.db"),
			ResourcesZip: getEnv("DATA_RESOURCES_ZIP", "./data/resources.zip"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
