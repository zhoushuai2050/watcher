# Watcher

单机监控面板：看资源、进程、监听端口，以及 SSH 爆破、Web 扫描、Fail2ban 封禁和连接异常。超过安全页设定的攻击阈值后，可用 nftables 永久封禁来源 IP。

Go 单二进制（采集 + API + 页面），数据存在 SQLite。适合 Linux 主机，建议用 systemd 跑、Nginx 反代。

## 功能

| 页面 | 做什么 |
|---|---|
| 总览 | 主机概况、打开的告警、最近安全事件 |
| 资源 | Tab 切换资源曲线和进程占用 |
| 网络 | 监听端口、连接状态 |
| 安全 | 攻击观测、白名单、永久封禁名单三个 Tab；列表支持模糊搜索和分页 |
| 设置 | 采集器状态、告警阈值、日志路径、改密码 |

默认账号 `admin` / `watcher`。**第一次登录后立刻改密码**，并改掉配置里的 `jwt_secret`。

## 编译

需要 Go 1.22+、Python 3。

```bash
git clone https://github.com/zhoushuai2050/watcher.git
cd watcher
make install    # 下载前端依赖、go mod tidy
make test
make build      # 打包前端并生成 bin/watcher
```

只跑本机开发：

```bash
make dev-api    # 默认 127.0.0.1:8020
make dev-web    # 开发页反代到上面的 API
```

浏览器打开开发页路径 `/Watcher/`。

## 安装到服务器

在目标机器上编译后：

```bash
sudo make deploy
```

脚本会：

1. 安装二进制到 `/usr/local/bin/watcher`
2. 写入示例配置 `/etc/watcher/config.toml`（已存在则不覆盖）
3. 数据目录 `/var/lib/watcher`
4. 启用 `watcher.service`（用户 `watcher`）
5. 若本机已有 Nginx 站点文件，尝试挂上 `/Watcher/` 反代

也可以只编译，再按 `deploy/install.sh`、`deploy/watcher.service` 自行安装。

装好后页面默认在：

```text
http://<主机>/Watcher/
```

服务只监听 `127.0.0.1:8020`，请用 Nginx 或其它反代对外提供 HTTPS。

### Nginx 反代示例

```nginx
location = /Watcher {
    return 301 /Watcher/;
}
location /Watcher/ {
    proxy_pass http://127.0.0.1:8020/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_http_version 1.1;
}
```

`public_path` 必须和反代前缀一致，默认 `/Watcher`。

## 配置

主配置：`/etc/watcher/config.toml`  
检测阈值也可在页面「设置」里改，保存在数据库，优先于文件里的 `[detect]`。

首次部署请至少改这几项：

```toml
listen = "127.0.0.1:8020"
data_dir = "/var/lib/watcher"
public_path = "/Watcher"
admin_user = "admin"
admin_password = "换成足够长的密码"
jwt_secret = "换成随机字符串"
jwt_days = 7

# 采集日志，按实际路径改
nginx_log = "/var/log/nginx/access.log"
fail2ban_log = "/var/log/fail2ban.log"
kern_log = "/var/log/kern.log"
# auth_log = "/var/log/auth.log"   # 不填则尝试 journalctl

# 允许存在的监听，格式 proto:port。白名单外的新端口会告警
listen_allow = ["tcp:22", "tcp:80", "tcp:443"]

[detect]
cpu_pct = 90
mem_pct = 90
disk_pct = 90
ssh_fail_count = 5
ssh_fail_window_sec = 600
web_probe_count = 10
attack_ban_threshold = 3
```

改完配置：

```bash
sudo systemctl restart watcher
```

### 读日志的权限

运行用户是 `watcher`。安装脚本会把它加入 `adm`、`systemd-journal`，并对 Nginx 日志尝试 `setfacl`。

若安全页没有 SSH / Nginx / Fail2ban 事件，到「设置」看采集器状态：

- 读不到 `auth.log`：把用户加入 `adm`，或确认能跑 `journalctl -u ssh`
- 读不到 Nginx 日志：给该用户读权限（ACL 或把日志组改成 `adm`）
- Fail2ban 同理，确认 `fail2ban_log` 路径正确

### 永久封禁的权限

自动/手动永久封禁走 nftables 表 `inet watcher`。systemd 单元需要 `CAP_NET_ADMIN`（`deploy/watcher.service` 已包含）。没有该能力时，封禁会记在库里但网卡规则加不上，采集器「ban」会报错。

## 怎么用

1. 打开 `/Watcher/`，用管理员账号登录。
2. **设置 → 修改密码**。默认密码不要留在生产环境。
3. **设置** 里核采集器是否全绿，按需要改 CPU/内存/SSH/Web 等阈值和监听白名单。
4. **总览 / 资源 / 网络** 看主机状态；资源页用 Tab 切换曲线和进程。
5. **安全**：按 IP 看失败类事件。把「攻击阈值」设成你能接受的次数（默认 3，0 表示关闭自动封禁）。同一 IP 在 7 天内失败类事件达到该次数，会全端口永久丢弃。本机、内网、Cloudflare 回源默认不自动封。常用出口加到「白名单」Tab，不会被永久封禁。
6. 误封自己的公网 IP：在安全页点「解封」，或在服务器上：

```bash
sudo watcher-unban <IP>
```

失败类事件包括：SSH 失败/无效用户、Web 探测、Web 登录失败、Fail2ban 封禁、端口扫描。阈值 3 比较严，建议把常用出口加进白名单。

## 告警规则

均可在设置页调整对应阈值。

| 规则 | 含义 |
|---|---|
| `ssh.bruteforce` | 窗口内同一 IP SSH 失败次数过多 |
| `ssh.invalid_user` | 扫不存在的用户 |
| `ssh.success_after_fail` | 失败多次后登录成功 |
| `ssh.new_source` | 从未见过的 IP 登录成功 |
| `web.probe` | 常见漏洞路径探测 |
| `web.bruteforce` | 登录路径 401/403 过密 |
| `fail2ban.ban` | Fail2ban 封禁 |
| `ip.permanent_ban` | 达到攻击阈值，已永久封禁 |
| `net.new_listen` | 监听白名单外的新端口 |
| `net.port_scan` | 短时间连接很多不同端口 |
| `net.conn_flood` | 单 IP 对单端口连接暴增 |
| `host.cpu` / `host.mem` / `host.disk` | 资源持续超阈值 |

## 日常维护

```bash
sudo systemctl status watcher
sudo journalctl -u watcher -f
```

数据文件在 `data_dir` 下的 `watcher.db`。升级：拉新代码后 `make build && sudo make deploy`（不会覆盖已有 `config.toml`）。
