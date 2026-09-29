package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var (
	Conf *Config
)

type Database struct {
	Host        string `mapstructure:"HOST"`
	Port        int    `mapstructure:"PORT"`
	User        string `mapstructure:"USER"`
	Password    string `mapstructure:"PASSWORD"`
	DBName      string `mapstructure:"NAME"`
	SSLMode     string `mapstructure:"SSL_MODE"`
	TablePrefix string `mapstructure:"TABLE_PREFIX"`
}

type Cors struct {
	AllowOrigins []string `json:"allow_origins" mapstructure:"ALLOW_ORIGINS"`
	AllowMethods []string `json:"allow_methods" mapstructure:"ALLOW_METHODS"`
	AllowHeaders []string `json:"allow_headers" mapstructure:"ALLOW_HEADERS"`
}

type LogConfig struct {
	Enable     bool   `mapstructure:"ENABLE"`
	Level      string `mapstructure:"LEVEL"`
	MaxSize    int    `mapstructure:"MAX_SIZE"`
	MaxAge     int    `mapstructure:"MAX_AGE"`
	MaxBackups int    `mapstructure:"MAX_BACKUPS"`
	Compress   bool   `mapstructure:"COMPRESS"`
	FilePath   string `mapstructure:"FILE_PATH"`
}

type EmailConfig struct {
	Host     string `mapstructure:"HOST"`
	Port     int    `mapstructure:"PORT"`
	Username string `mapstructure:"USERNAME"`
	Password string `mapstructure:"PASSWORD"`
	From     string `mapstructure:"FROM"`
}

type RateLimitConfig struct {
	RequestsPerSecond int           `mapstructure:"REQUESTS_PER_SECOND"`
	Burst             int           `mapstructure:"BURST"`
	BlockDuration     time.Duration `mapstructure:"BLOCK_DURATION"`
}

type WechatConfig struct {
	AppID     string   `mapstructure:"APP_ID"`
	AppSecret string   `mapstructure:"APP_SECRET"`
	Token     string   `mapstructure:"TOKEN"`
	MasterIDs []string `mapstructure:"MASTER_IDS"`
}

type ModerationConfig struct {
	Enabled         bool          `mapstructure:"ENABLED"`
	ReviewRequired  bool          `mapstructure:"REVIEW_REQUIRED"`
	BaseURL         string        `mapstructure:"BASE_URL"`
	Token           string        `mapstructure:"TOKEN"`
	SensitiveWords  []string      `mapstructure:"SENSITIVE_WORDS"`
	Timeout         time.Duration `mapstructure:"TIMEOUT"`
	RetryInterval   time.Duration `mapstructure:"RETRY_INTERVAL"`
	QueueSize       int           `mapstructure:"QUEUE_SIZE"`
	BatchSize       int           `mapstructure:"BATCH_SIZE"`
	ReportThreshold int           `mapstructure:"REPORT_THRESHOLD"`
}

type Config struct {
	Port        int              `mapstructure:"PORT"`
	JwtSecret   string           `mapstructure:"JWT_SECRET"`
	TokenExpire int              `mapstructure:"TOKEN_EXPIRE"`
	Database    Database         `mapstructure:"DATABASE"`
	LogConfig   LogConfig        `mapstructure:"log_config"`
	Cors        Cors             `mapstructure:"cors"`
	Email       EmailConfig      `mapstructure:"email"`
	RateLimit   RateLimitConfig  `mapstructure:"rate_limit"`
	Wechat      WechatConfig     `mapstructure:"wechat"`
	Moderation  ModerationConfig `mapstructure:"moderation"`
}

func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetConfigName(configNameByEnv())
	v.SetConfigType("toml")
	addConfigPaths(v)

	// 配置里不写 REVIEW_REQUIRED 时，保持"先审后发"的旧行为
	v.SetDefault("moderation.review_required", true)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found")
		}
		return nil, fmt.Errorf("config file read error: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	return &cfg, nil
}

func configNameByEnv() string {
	switch strings.ToLower(os.Getenv("PKUPHYSU_ENV")) {
	case "test":
		return "config.test"
	case "dev":
		return "config.dev"
	default: // prod 或未设置
		return "config"
	}
}

func addConfigPaths(v *viper.Viper) {
	// 常见相对路径（覆盖从项目根目录和包目录运行测试的场景）
	v.AddConfigPath("./data/config")
	v.AddConfigPath("../data/config")
	v.AddConfigPath("../../data/config")
	v.AddConfigPath("../../../data/config")

	// 从当前工作目录逐级向上搜索 data/config，提升在不同执行目录下的稳定性。
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "data", "config")
		if stat, statErr := os.Stat(candidate); statErr == nil && stat.IsDir() {
			v.AddConfigPath(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
}

func InitConfig() {
	cfg, err := LoadConfig()
	if err != nil {
		panic(err)
	}
	Conf = cfg
}
