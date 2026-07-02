#!/bin/bash
# devrec — Linux test session recorder installer
# 全自动环境检测 + Go 安装 + 二进制部署
#
# 获取二进制优先级：
#   1. ./devrec-linux-amd64 / ./devrec-linux-arm64 — 离线/本地产物
#   2. ./main.go + Go 编译器 → go build（保证最新）
#   3. GitHub Releases 预编译二进制下载
#   4. ./main.go 无 Go → 自动安装 Go 1.24 → go build
#   5. GitHub Releases 源码下载 → 自动装 Go → go build
#
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/0embsd/devrec/main/install.sh | sudo bash
#   sudo bash install.sh              # 本地运行
#   DEVREC_VERSION=latest sudo bash install.sh  # 指定版本

set -euo pipefail

REPO="0embsd/devrec"
BIN="/usr/local/bin/devrec"
DIR="/opt/devrec"
PIDDIR="/var/run/devrec"
GO_VERSION="1.24.0"
GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TAR}"

# ─── 颜色 ───
C_RESET="" C_BOLD="" C_CYAN="" C_GREEN="" C_RED=""
if [[ -t 1 ]]; then
    C_RESET=$'\033[0m' C_BOLD=$'\033[1m'
    C_CYAN=$'\033[36m' C_GREEN=$'\033[32m' C_RED=$'\033[31m'
fi

echo "╔══════════════════════════════════╗"
echo "║   devrec 一键安装 (Go)            ║"
echo "╚══════════════════════════════════╝"
echo ""

# ── 权限检查 ──
if [[ "$(id -u)" -ne 0 ]]; then
    echo "❌ 需要 root 权限，请使用 sudo bash install.sh"
    exit 1
fi

# ── 架构检测 ──
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  GOARCH="amd64"; BINARY="devrec-linux-amd64" ;;
    aarch64|arm64) GOARCH="arm64"; BINARY="devrec-linux-arm64"
                   GO_TAR="go${GO_VERSION}.linux-arm64.tar.gz"
                   GO_URL="https://go.dev/dl/${GO_TAR}" ;;
    *) echo "❌ 不支持的架构: $ARCH"; exit 1 ;;
esac

# ── 检查基础依赖 ──
ensure_cmd() {
    local cmd="$1" pkg="${2:-$1}"
    if ! command -v "$cmd" &>/dev/null; then
        echo "→ 安装缺失依赖: $pkg..."
        apt-get update -qq 2>/dev/null || true
        apt-get install -y -qq "$pkg" 2>/dev/null || {
            echo "⚠ 无法安装 $pkg，部分功能可能受限"
        }
    fi
}

ensure_cmd curl
ensure_cmd tar

# ── 安装 Go 编译器（需要时）──
install_go() {
    if command -v /usr/local/go/bin/go &>/dev/null; then
        local installed_ver
        installed_ver=$(/usr/local/go/bin/go version | awk '{print $3}' | sed 's/^go//')
        if [[ "$(printf '%s\n' "$GO_VERSION" "$installed_ver" | sort -V | head -1)" == "$GO_VERSION" ]]; then
            export PATH="/usr/local/go/bin:$PATH"
            return 0
        fi
        echo "⚠ /usr/local/go 版本过旧 (go$installed_ver < go$GO_VERSION)，将覆盖升级"
    fi
    echo "→ 安装 Go ${GO_VERSION}（${GOARCH}）..."
    curl -fsSL --max-time 120 "$GO_URL" -o "/tmp/${GO_TAR}" || {
        echo "⚠ go.dev 下载失败，尝试 goproxy.cn 镜像..."
        curl -fsSL --max-time 120 "https://goproxy.cn/dl/${GO_TAR}" -o "/tmp/${GO_TAR}" || {
            echo "❌ Go 编译器下载失败"
            return 1
        }
    }
    echo "→ 校验 SHA256..."
    local expected_sum
    expected_sum=$(curl -fsSL --max-time 10 "https://go.dev/dl/${GO_TAR}.sha256" 2>/dev/null | awk '{print $1}' || true)
    if [[ -n "$expected_sum" ]]; then
        local actual_sum
        actual_sum=$(sha256sum "/tmp/${GO_TAR}" | awk '{print $1}')
        if [[ "$actual_sum" != "$expected_sum" ]]; then
            echo "❌ Go 下载文件 SHA256 校验失败"
            echo "   预期: $expected_sum"
            echo "   实际: $actual_sum"
            rm -f "/tmp/${GO_TAR}"
            return 1
        fi
        echo "✓ SHA256 校验通过"
    else
        echo "⚠ 无法获取官方 SHA256 指纹，跳过校验"
    fi
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm -f "/tmp/${GO_TAR}"
    export PATH="/usr/local/go/bin:$PATH"
    echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/go.sh
    echo "✓ Go $(/usr/local/go/bin/go version | awk '{print $3}') 已安装"
}

# ══════════════════════════════════════
# 主流程：获取 devrec 二进制
# ══════════════════════════════════════

RANK=""

# 优先级 1：本地预编译产物
if [[ -f "./${BINARY}" ]]; then
    RANK="[离线: 预编译 ${BINARY}]"
    echo "→ ${RANK}"
    rm -f "$BIN"
    cp "./${BINARY}" "$BIN"
    chmod 755 "$BIN"

# 优先级 2：有源码 → 编译
elif [[ -f "./main.go" ]]; then
    if command -v go &>/dev/null || [[ -x /usr/local/go/bin/go ]]; then
        RANK="[编译: 源码 + Go]"
    else
        RANK="[编译: 源码 + 自动装 Go]"
        install_go || exit 1
    fi
    echo "→ ${RANK}"
    export PATH="/usr/local/go/bin:$PATH"
    go build -buildvcs=false -ldflags="-s -w" -o /tmp/devrec-build . || {
        echo "❌ 编译失败"; exit 1
    }
    rm -f "$BIN"
    cp /tmp/devrec-build "$BIN"
    chmod 755 "$BIN"
    rm -f /tmp/devrec-build

# 优先级 3：GitHub Releases 预编译二进制
else
    VERSION="${DEVREC_VERSION:-latest}"
    if [ "$VERSION" = "latest" ]; then
        RELEASE_URL="https://github.com/$REPO/releases/latest/download/$BINARY"
    else
        RELEASE_URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY"
    fi
    echo "→ [在线] GitHub Releases: $RELEASE_URL"
    if curl -fsSL --connect-timeout 15 --max-time 60 -o "$BIN" "$RELEASE_URL" 2>/dev/null; then
        chmod 755 "$BIN"
        file "$BIN" | grep -q "ELF" && RANK="[在线: GitHub Release]" && echo "→ ${RANK}"
    fi

    # 优先级 4：Releases 无预编译包 → 下载源码 + 装 Go + 编译
    if [[ "$RANK" == "" ]]; then
        echo "→ GitHub Releases 无预编译包，从源码编译..."
        tmp_dir=
        tmp_dir="$(mktemp -d /tmp/devrec-build.XXXXXX)"
        cd "$tmp_dir"

        echo "→ 下载源码: https://github.com/$REPO/archive/refs/heads/main.tar.gz"
        curl -fsSL --max-time 60 -o source.tar.gz "https://github.com/$REPO/archive/refs/heads/main.tar.gz" || {
            echo "❌ 源码下载失败"
            cd / && rm -rf "$tmp_dir"
            exit 1
        }

        tar -xzf source.tar.gz
        cd devrec-* 2>/dev/null || cd devrec-main 2>/dev/null || cd */ 2>/dev/null

        if [[ ! -f "main.go" ]]; then
            echo "❌ 源码包中未找到 main.go"
            cd / && rm -rf "$tmp_dir"
            exit 1
        fi

        install_go || { cd / && rm -rf "$tmp_dir"; exit 1; }

        echo "→ 编译 devrec..."
        if ! go build -buildvcs=false -ldflags="-s -w" -o devrec . 2>&1; then
            echo "❌ 编译失败"
            cd / && rm -rf "$tmp_dir"
            exit 1
        fi
        rm -f "$BIN"
        cp devrec "$BIN"
        chmod 755 "$BIN"
        cd / && rm -rf "$tmp_dir"
        RANK="[编译: 在线源码 + Go]"
        echo "→ ${RANK}"
    fi
fi

echo "Installed: $BIN"

# ── 创建运行时目录 ──
mkdir -p "$DIR/sessions" "$DIR/tmp" "$PIDDIR"
chmod 755 "$PIDDIR"

# ── 运行时依赖检查 ──
check_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "⚠ $1 未找到，请安装: apt install $2"
    fi
}
check_cmd zstd zstd
check_cmd script util-linux
check_cmd scriptreplay util-linux
check_cmd ss iproute2
check_cmd openssl openssl

echo ""
echo "=== devrec 安装完成 ==="
echo ""
echo "快速上手:"
echo "  devrec start -t 'my-test' -c kernel,resources,ports"
echo "  devrec stop"
echo "  devrec replay <session-id>"
echo "  devrec watch --interval 30s"
echo "  devrec list"
echo "  devrec status"
"$BIN" --help 2>/dev/null || true
