# devrec

Linux 测试会话录制工具 — 捕获终端输出 + 系统快照 + 结构化 diff，生成 zstd 压缩归档供事后分析。

灵感来自 [asciinema](https://github.com/asciinema/asciinema)、[vhs](https://github.com/charmbracelet/vhs)、[t-rec-rs](https://github.com/sassman/t-rec-rs)。

## 安装

### 公共安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/0embsd/devrec/main/install.sh | sudo bash
```

install.sh 自动检测环境：无 Go → 安装 Go 1.24 → 编译；有本地二进制 → 直接安装。支持 5 级 fallback 优先级链。

### 私有安装（开发者）

```bash
git clone git@github.com:0embsd/devrec.git
cd devrec
make build && sudo make install
```

### 系统依赖

`zstd` `curl` `tar` `script` `scriptreplay` `ss` `openssl`

install.sh 会自动安装缺失的系统包；`make install` 需自行确保依赖就绪。

---

## 快速开始

```bash
# 启动录制（9 个内置 collector 可选）
sudo devrec start -t "my-test" -c kernel,resources,ports

# 执行测试操作... 按 Ctrl+D 结束

# 查看历史
sudo devrec list

# 回放（时序还原）
sudo devrec replay <session-id>
```

## 命令

| 命令 | 功能 |
|------|------|
| `devrec start -t TAG -c C1,C2,... [-s SHELL]` | 启动录制 session |
| `devrec stop [-s ID]` | 停止录制并归档 |
| `devrec status` | 当前会话状态 + 最近 3 个归档 |
| `devrec list [-n N] [--json]` | 历史会话列表 |
| `devrec replay <id>` | scriptreplay 时序回放 |
| `devrec watch [-i 30s] [-d 2h] [-c C1,C2] [-t TAG]` | 纯快照模式，定时采集 |
| `devrec cleanup [-k 20] [--dry-run]` | 清理旧归档 |

## 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `DEVREC_DIR` | 归档存储目录 | `/opt/devrec` |
| `DEVREC_SHELL` | 录制 shell | `/bin/bash` |
| `DEVREC_COLLECTORS` | 默认 collector 列表 | `systemd,ports,network` |
| `DEVREC_KEEP` | 保留归档数 | `20` |
| `DEVREC_TIMEOUT` | collector 超时 | `15s` |
| `DEVREC_CONFIG` | 配置文件路径（优先级最高） | `/etc/devrec.yaml` |

优先级: CLI flags > 环境变量 > `~/.devrec.yaml` > `/etc/devrec.yaml` > defaults

## 内置 Collector

| Collector | 采集内容 |
|-----------|---------|
| `kernel` | `/etc/os-release` + `uname -a` |
| `resources` | `df` / `free` / `uptime` / `/proc/loadavg` |
| `systemd` | `systemctl is-active` 指定 unit（默认: xray, nginx, ssh, ufw） |
| `ports` | `ss -tlnp` 监听端口 |
| `network` | `ip -j addr show` 网卡信息 |
| `cert` | `openssl x509` 证书过期检查（自动检测路径） |
| `firewall` | `ufw status` + `iptables -L` |
| `security` | `getenforce` + `aa-status` |
| `custom` | 用户自定义命令：`--collectors custom:label=cmd` |

## 配置文件 (~/.devrec.yaml)

```yaml
# 可选，每项都有默认值
dir: /opt/devrec
shell: /bin/bash
default_collectors: systemd,ports,network,resources,firewall,kernel
keep_archives: 20
collector_timeout: 15s
pid_dir: /var/run/devrec
systemd_units: xray,nginx,ssh,ufw
cert_paths:                               # 留空 = 自动检测
```

优先级: CLI flags > 环境变量 > `~/.devrec.yaml` > `/etc/devrec.yaml` > defaults

## 归档结构

```
/opt/devrec/sessions/<uuid>.tar.zst       # zstd 可用时，否则 .tar.gz
  ├── session.json       # 元数据（ID、tag、时间、用户）
  ├── pre.json           # 测试前系统快照
  ├── post.json          # 测试后系统快照
  ├── diff.json          # 结构化差异对比（字段级粒度）
  ├── terminal.log       # 终端原始输出
  └── terminal.time      # 时序文件（scriptreplay 用）
```

## 设计原则

- **标准库优先** — 仅 2 个外部依赖（cobra + pflag），zstd 压缩调用系统二进制
- **独立生命周期** — 不绑定任何被测试项目
- **信号安全** — SIGINT/SIGTERM/SIGHUP 均触发完整清理 + 归档
- **UUID 会话 ID** — 稳定不变，跨 start/stop/status 一致
- **zstd 压缩** — fallback 到 gzip 如果 zstd 未安装
- **5 级安装链** — 本地二进制 → 源码编译 → GitHub Release → 源码+自动装Go → GitHub源码+自动装Go
