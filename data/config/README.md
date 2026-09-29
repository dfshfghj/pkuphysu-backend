# 配置文件说明

## 文件结构

```
data/config/
├── config.toml          # 生产环境配置文件（请妥善保管，不要提交到 Git）
├── config.dev.toml      # 开发环境配置文件（可以提交到 Git）
├── config.test.toml     # 测试环境配置文件（可以提交到 Git）
└── README.md            # 本说明文档
```

## 配置文件说明

### config.toml - 生产环境配置

- **用途**：用于生产环境（本地开发请用 config.dev.toml）
- **数据库**：连接正式数据库
- **密钥**：使用正式的 JWT 密钥
- **注意**：此文件包含敏感信息，不应提交到版本控制系统

### config.dev.toml - 开发环境配置

- **用途**：本地开发，与生产环境完全隔离
- **端口**：`8082`（生产为 `8080`），可以和线上服务同时运行
- **数据库**：`pkuphysu_dev`（生产为 `pkuphysu_db`，同一个 PostgreSQL 实例、不同的库）
- **密钥**：使用开发专用的 JWT 密钥，线上签发的 token 在 dev 无效，反之亦然
- **邮件/微信**：不接入真实服务，避免向真实用户发送邮件或消息
- **注意**：不包含任何生产密钥，可以安全地提交到 Git

### config.test.toml - 测试环境配置

- **用途**：专门用于自动化测试
- **数据库**：连接测试数据库（`pkuphysu_test`）
- **密钥**：使用测试专用的 JWT 密钥
- **日志**：禁用日志输出以提高测试速度
- **注意**：此文件可以安全地提交到 Git

## 使用方法

### 生产环境

```bash
# 默认使用 config.toml
go run ./cmd

# 或者明确指定
PKUPHYSU_ENV=prod go run ./cmd
```

### 开发环境

```bash
# 1. 首次使用：创建开发库和开发用户（幂等，可重复执行）
make db-dev-setup

# 2. 可选：把生产数据导入开发库（生产库只读，开发库会被重建）
make db-dev-sync

# 3. 启动开发后端（端口 8082）
make run-dev
```

### 测试环境

```bash
# 使用 Makefile
make test

# 或手动指定环境变量
PKUPHYSU_ENV=test go test ./...
```

## 环境变量

| 变量名 | 值 | 使用的配置文件 |
|--------|-----|---------------|
| `PKUPHYSU_ENV` | 未设置 | `config.toml` |
| `PKUPHYSU_ENV` | `prod` | `config.toml` |
| `PKUPHYSU_ENV` | `dev` | `config.dev.toml` |
| `PKUPHYSU_ENV` | `test` | `config.test.toml` |

## 环境隔离说明

三个环境互不影响：

| | 生产 | 开发 | 测试 |
|---|---|---|---|
| 配置文件 | `config.toml` | `config.dev.toml` | `config.test.toml` |
| 端口 | 8080 | 8082 | 8080（测试不监听） |
| 数据库 | `pkuphysu_db` | `pkuphysu_dev` | `pkuphysu_test` |
| 数据库用户 | `pkuphysu` | `pkuphysu_dev` | `pkuphysu_test` |

### 生产数据同步到开发库

```bash
./scripts/sync_prod_to_dev.sh                 # 全量：表结构 + 数据
./scripts/sync_prod_to_dev.sh --schema-only   # 只同步表结构
./scripts/sync_prod_to_dev.sh --dry-run       # 只打印将要执行的操作
```

- 生产库只做只读导出（`pg_dump`），不会修改线上数据
- 开发库会被整体重建，本地开发数据会丢失
- 脚本会校验两侧的表行数是否一致
- 同步后开发库里包含真实用户数据，请只在本地使用

### 开发环境注意事项

- 邮件/微信未配置：注册验证码不会真的发邮件，但会写入数据库，可直接查询
  `SELECT email, code FROM pkuphysu_email_verifications WHERE used = false;`
- 上传的文件仍写入 `data/static`、`data/avatar`（与生产共用目录），如需隔离后续可再拆分

## 配置项说明

### 主要配置

- `PORT`: 服务器端口
- `JWT_SECRET`: JWT 签名密钥（重要！请修改为随机字符串）
- `TOKEN_EXPIRE`: Token 过期时间（小时）

### 数据库配置

- `HOST`: 数据库主机
- `PORT`: 数据库端口
- `USER`: 数据库用户名
- `PASSWORD`: 数据库密码
- `NAME`: 数据库名称
- `SSL_MODE`: SSL 模式（通常为 `disable`）
- `TABLE_PREFIX`: 表名前缀

### 内容审核配置（`[moderation]`）

- `REVIEW_REQUIRED`: `true`（默认，配置里不写时也是 true）新帖子和评论先进入待审核状态，审核通过后才公开，即"先审后发"；`false` 时发布即为可见，不经过审核流程
- `ENABLED`: 是否接入智谱内容审核接口，开启后待审核内容会送审，`PASS` 自动通过，否则转人工复核
- `BASE_URL` / `TOKEN`: 审核接口地址和密钥
- `SENSITIVE_WORDS`: 命中敏感词的内容会在提交时直接拒绝
- 其余为队列、超时、重试等参数

### 其他配置

- `log_config`: 日志配置
- `cors`: CORS 跨域配置
- `email`: 邮件服务配置
- `rate_limit`: 请求频率限制配置

## 安全提示

⚠️ **重要**：

1. **不要**将 `config.toml` 提交到 Git
2. 在生产环境中使用强密码和随机 JWT 密钥
3. 定期更换敏感配置
4. 限制数据库用户的权限

## .gitignore 配置

确保 `.gitignore` 中包含：

```gitignore
# 生产环境配置文件
data/config/config.toml

# 但保留开发、测试配置
!data/config/config.dev.toml
!data/config/config.test.toml
```

## 示例配置生成

可以使用以下命令生成安全的 JWT 密钥：

```bash
# Linux/Mac
openssl rand -base64 32

# 或使用 Go
go run -e 'crypto/rand' -e 'encoding/base64' - <<'EOF'
package main
import ("crypto/rand"; "encoding/base64"; "fmt")
func main() {
    b := make([]byte, 32)
    rand.Read(b)
    fmt.Println(base64.StdEncoding.EncodeToString(b))
}
EOF
```

---

**最后更新**: 2026-09-29
