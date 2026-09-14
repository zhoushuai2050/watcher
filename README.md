# Watcher

本机服务器监控：资源、进程占用、监听端口，以及 SSH 爆破、Web 扫描、Fail2ban 封禁和连接异常。

- 后端：Go 单二进制（采集 + API + 静态页）
- 前端：React + TypeScript（esbuild 打包）
- 存储：SQLite
- 运行：systemd，Nginx 反代 `/Watcher/`

安全页可设攻击阈值（默认 3）：同一 IP 7 天内失败类事件达到该次数后，nftables 永久全端口封禁。

## 本地开发

```bash
make install          # vendor 前端 + go mod tidy
make test
make build            # 打前端并编译 bin/watcher
make dev-api          # 127.0.0.1:8020
make dev-web          # 127.0.0.1:5174/Watcher/  反代 API
```

默认账号 `admin` / `watcher`，登录后马上改密码。

## 发布到本机

```bash
make deploy           # 编译、安装 systemd、尝试挂 Nginx
```

- 页面：`https://zhoushuai.duckdns.org/Watcher/`
- 接口：`https://zhoushuai.duckdns.org/Watcher/api/`
- 数据：`/var/lib/watcher/watcher.db`
- 配置：`/etc/watcher/config.toml`
- systemd：`watcher.service`

运行用户 `watcher` 加入 `adm`、`systemd-journal`。读不到 Nginx 日志时，安装脚本会尝试 `setfacl`。

## 接口

统一响应：

```json
{ "code": 0, "message": "ok", "data": {} }
```

- `POST /api/v1/auth/login`
- `GET /api/v1/me`
- `GET /api/v1/overview`
- `GET /api/v1/metrics?range=1h|6h|24h|7d`
- `GET /api/v1/processes`
- `GET /api/v1/network`
- `GET /api/v1/security/events`
- `GET /api/v1/security/ips`
- `GET|POST /api/v1/security/bans`
- `POST /api/v1/security/unban`
- `GET /api/v1/alerts`
- `POST /api/v1/alerts/ack` `{ids}`
- `POST /api/v1/alerts/resolve` `{ids}`
- `GET|PUT /api/v1/settings`

经 Nginx 时前缀为 `/Watcher`。

## 告警规则（默认可改）

| 规则 | 含义 |
|---|---|
| `ssh.bruteforce` | 10 分钟内同一 IP SSH 失败 ≥ 8 |
| `ssh.invalid_user` | 扫不存在用户 |
| `ssh.success_after_fail` | 失败后成功登入 |
| `ssh.new_source` | 新 IP 登录成功 |
| `web.probe` | 常见漏洞路径探测 |
| `web.bruteforce` | 登录路径 401/403 密集 |
| `fail2ban.ban` | Fail2ban 封禁 |
| `ip.permanent_ban` | 超过攻击阈值后永久封禁 |
| `net.new_listen` | 白名单外的新监听端口 |
| `net.port_scan` | 短时间打很多不同端口 |
| `net.conn_flood` | 单 IP 对单端口连接暴增 |
| `host.cpu` / `host.mem` / `host.disk` | 资源超阈值 |
