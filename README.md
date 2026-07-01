# devrec

Linux 测试会话录制工具 — 捕获终端输出 + 系统快照 + 结构化 diff，生成 zstd 压缩归档供事后分析。

灵感来自 [asciinema](https://github.com/asciinema/asciinema)、[vhs](https://github.com/charmbracelet/vhs)、[t-rec-rs](https://github.com/sassman/t-rec-rs)。

## 安装

```bash
# 系统依赖: zstd (apt install zstd)
git clone git@github.com:0embsd/devrec.git /tmp/devrec
cd /tmp/devrec
sudo make install
```

## 快速开始

```bash
# 启动录制 (8 个内置 collector 可选)
sudo devrec start -t "my-test" -c kernel,resources,ports

# 执行测试操作... 按 Ctrl+D 结束

# 查看历史
sudo devrec list

# 回放 (时序还原)
sudo devrec replay <session-id>
```

## 命令

| 命令 | 功能 |
|------|------|
| `devrec start -t TAG -c C1,C2,...` | 启动录制 session |
| `devrec stop [-s ID]` | 停止录制并归档 |
| `devrec status` | 当前会话状态 + 最近 3 个归档 |
| `devrec list [-n N] [--json]` | 历史会话列表 |
| `devrec replay <id>` | scriptreplay 时序回放 |
| `devrec watch [-i 30s] [-d 2h]` | 纯快照模式，定时采集 |
| `devrec cleanup [-k 20] [--dry-run]` | 清理旧归档 |

## 内置 Collector

| Collector | 采集内容 |
|-----------|---------|
| `kernel` | `/etc/os-release` + `uname -a` |
| `resources` | `df` / `free` / `uptime` / `/proc/loadavg` |
| `systemd` | `systemctl is-active` 指定 unit |
| `ports` | `ss -tlnp` 监听端口 |
| `network` | `ip -j addr show` 网卡信息 |
| `cert` | `openssl x509` 证书过期检查 |
| `firewall` | `ufw status` + `iptables -L` |
| `security` | `getenforce` + `aa-status` |

## 配置文件 (~/.devrec.yaml)

```yaml
# 可选，每项都有默认值
dir: /opt/devrec
shell: /bin/bash
default_collectors: systemd,ports,network,resources,firewall,kernel
keep_archives: 20
collector_timeout: 15s
systemd_units: xray,nginx,ssh,ufw
cert_paths: /etc/ssl/certs,/etc/nginx/ssl
```

优先级: CLI flags > 环境变量 > `~/.devrec.yaml` > `/etc/devrec.yaml` > defaults

## 归档结构

```
/opt/devrec/sessions/<uuid>.tar.zst
  ├── session.json       # 元数据（ID、tag、时间、用户）
  ├── pre.json           # 测试前系统快照
  ├── post.json          # 测试后系统快照
  ├── diff.json          # 结构化差异对比
  ├── terminal.log       # 终端原始输出
  └── terminal.time      # 时序文件（scriptreplay 用）
```

## 设计原则

- **零外部 Go 依赖** — 标准库 + cobra/pflag；zstd 压缩调用系统二进制
- **独立生命周期** — 不绑定任何被测试项目
- **信号安全** — SIGINT/SIGTERM/SIGHUP 均触发完整清理 + 归档
- **UUID 会话 ID** — 稳定不变，跨 start/stop/status 一致
- **zstd 压缩** — 比 gzip 小 ~8%，fallback 到 gzip 如果 zstd 未安装
