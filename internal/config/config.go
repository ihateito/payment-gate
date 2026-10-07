package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds unified application configuration for services.
type Config struct {
	AppEnv    string
	LogLevel  string
	LogFormat string

	// HTTP Gateway
	HTTPPort string

	// PostgreSQL
	DatabaseURL     string
	DBMaxConns      int32
	DBMinConns      int32
	DBHealthTimeout time.Duration

	// Redis
	RedisAddr     string
	RedisPassword string

	// Kafka / Redpanda
	KafkaBrokers      string
	KafkaPaymentTopic string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		AppEnv:            getEnv("APP_ENV", "development"),
		LogLevel:          getEnv("LOG_LEVEL", "DEBUG"),
		LogFormat:         getEnv("LOG_FORMAT", "json"),
		HTTPPort:          getEnv("GATEWAY_HTTP_PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://payment_user:payment_pass@localhost:5432/payment_gateway?sslmode=disable"),
		DBMaxConns:        getInt32Env("DB_MAX_CONNS", 25),
		DBMinConns:        getInt32Env("DB_MIN_CONNS", 5),
		DBHealthTimeout:   3 * time.Second,
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaPaymentTopic: getEnv("KAFKA_TOPIC_PAYMENT_EVENTS", "payment.events.v1"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getInt32Env(key string, defaultVal int32) int32 {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 32); err == nil {
			return int32(parsed)
		}
	}
	return defaultVal
}
