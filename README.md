# devrec

Linux 测试会话录制工具 — 捕获终端输出 + 系统快照 + 结构化 diff，生成 tar.gz 归档供事后分析。

## 安装

```bash
# 私有仓库，用 git clone + make install
git clone git@github.com:0embsd/devrec.git /tmp/devrec
cd /tmp/devrec
sudo make install
```

## 快速开始

```bash
# 启动录制
sudo devrec start -t "my-test" -c kernel,resources,ports

# 在录制终端中执行测试操作...

# 按 Ctrl+D 或输入 exit 结束录制
# 或从另一个终端：
sudo devrec stop

# 查看录制历史
sudo devrec list

# 回放
sudo devrec replay <session-id>
```

## 命令

| 命令 | 功能 |
|------|------|
| `devrec start -t TAG -c C1,C2` | 启动录制，8 个内置 collector 可选 |
| `devrec stop` | 停止录制并打包归档 |
| `devrec status` | 当前会话状态 + 最近 3 个归档 |
| `devrec list -n N` | 历史会话列表 |
| `devrec replay <id>` | 用 scriptreplay 回放终端会话 |
| `devrec watch -i 30s -d 2h` | 纯快照模式，定时采集不录终端 |
| `devrec cleanup -k 20` | 保留最近 K 个归档，`--dry-run` 预览 |

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

## 归档结构

```
/opt/devrec/sessions/<uuid>.tar.gz
  ├── session.json       # 元数据（ID、tag、时间、用户）
  ├── pre.json           # 测试前系统快照
  ├── post.json          # 测试后系统快照
  ├── diff.json          # 结构化差异对比
  ├── terminal.log       # 终端原始输出
  └── terminal.time      # 时序文件（scriptreplay 用）
```

## 设计原则

- **零外部依赖** — 仅 Go 标准库 + cobra/pflag
- **独立生命周期** — 不绑定任何被测试项目
- **信号安全** — SIGINT/SIGTERM/SIGHUP 均触发完整清理 + 归档
- **UUID 会话 ID** — 稳定不变，跨 `start`/`stop`/`status` 一致
