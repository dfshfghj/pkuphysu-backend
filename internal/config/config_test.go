package config

import (
	"os"
	"testing"
)

func TestConfigSwitching(t *testing.T) {
	// 保存原环境变量
	originalEnv := os.Getenv("PKUPHYSU_ENV")
	defer os.Setenv("PKUPHYSU_ENV", originalEnv)

	// 测试加载 test 配置
	os.Setenv("PKUPHYSU_ENV", "test")
	Conf = nil // 重置全局变量

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	t.Logf("✓ Successfully loaded test config")
	t.Logf("  Database User: %s", cfg.Database.User)
	t.Logf("  Database Name: %s", cfg.Database.DBName)
	t.Logf("  JWT Secret: %s", cfg.JwtSecret)

	if cfg.Database.DBName != "pkuphysu_test" {
		t.Errorf("Expected test database name 'pkuphysu_test', got '%s'", cfg.Database.DBName)
	}
}
