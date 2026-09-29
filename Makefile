.PHONY: test test-verbose test-cover build run run-dev clean db-dev-setup db-dev-sync

# 运行所有测试（使用测试配置文件）
test:
	PKUPHYSU_ENV=test go test ./...

# 运行测试并显示详细信息
test-verbose:
	PKUPHYSU_ENV=test go test -v ./...

# 运行测试并生成覆盖率报告
test-cover:
	PKUPHYSU_ENV=test go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 构建项目
build:
	go build -o bin/server ./cmd

# 运行主程序（生产配置，端口 8080）
run:
	go run ./cmd

# 以开发环境运行（config.dev.toml，端口 8082 + 独立数据库 pkuphysu_dev）
run-dev:
	PKUPHYSU_ENV=dev go run ./cmd

# 创建开发环境数据库和用户（幂等，可重复执行）
db-dev-setup:
	./scripts/setup_dev_db.sh

# 把生产数据同步到开发库（只读导出生产库，重建开发库）
db-dev-sync:
	./scripts/sync_prod_to_dev.sh

# 清理
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
