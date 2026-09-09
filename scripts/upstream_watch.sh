#!/usr/bin/env bash
# CLIProxyAPI 上游巡检（只检查，不合并、不构建）
# 无更新：静默退出（空 stdout）
# 有更新 / 异常：输出中文摘要，由 Hermes cron 推送到指定群组/话题
set -u
export PATH="/usr/local/bin:/usr/bin:/bin:$PATH"
export LANG=C.UTF-8
export LC_ALL=C.UTF-8

REPO_DIR="/www/234G/CLIProxyAPI-开发"
GIT="/usr/bin/git"

# 针对本次 Antigravity 修复的核心文件模式
ANTIGRAVITY_PATTERNS=(
  '^internal/runtime/executor/antigravity_'
  '^internal/translator/antigravity/'
)

die_alert() {
  echo "⚠️ CLIProxyAPI 上游巡检异常"
  echo
  printf '%s\n' "$@"
  exit 0
}

if [ ! -d "$REPO_DIR/.git" ]; then
  die_alert "工作区目录不存在或不是 git 仓库: $REPO_DIR"
fi

cd "$REPO_DIR" || die_alert "无法进入工作区: $REPO_DIR"

HEAD="$($GIT rev-parse --short HEAD 2>/dev/null || true)"
if [ -z "$HEAD" ]; then
  die_alert "无法读取本地 HEAD"
fi

# 检查本地是否有未提交的修改
DIRTY_COUNT="$($GIT status --porcelain 2>/dev/null | grep -v '^\?\?' | wc -l | tr -d ' ')"
DIRTY_COUNT="${DIRTY_COUNT:-0}"

# 拉取 upstream
FETCH_ERR=""
if ! $GIT fetch upstream >/tmp/cpa_fetch_upstream.log 2>&1; then
  FETCH_ERR="upstream fetch 失败: $(head -n 3 /tmp/cpa_fetch_upstream.log 2>/dev/null)"
fi

if [ -n "$FETCH_ERR" ]; then
  die_alert \
    "当前本地 HEAD: $HEAD" \
    "远端拉取失败: $FETCH_ERR" \
    "请检查网络或 GitHub 连通性。"
fi

if ! $GIT rev-parse --verify upstream/main >/dev/null 2>&1; then
  die_alert "当前本地 HEAD: $HEAD" "找不到 upstream/main 分支，请检查 remote 配置"
fi

UPSTREAM_HEAD="$($GIT rev-parse --short upstream/main)"
NEW_COUNT="$($GIT rev-list --count HEAD..upstream/main 2>/dev/null || echo 0)"
NEW_COUNT="${NEW_COUNT:-0}"

# 无上游更新 → 静默退出
if [ "$NEW_COUNT" = "0" ]; then
  exit 0
fi

# 获取更新日志与改动文件
LOG_LINES="$($GIT log --oneline --date=short --format='%h %ad %s' HEAD..upstream/main 2>/dev/null | head -15)"
CHANGED_FILES="$($GIT diff --name-only HEAD...upstream/main 2>/dev/null || true)"
FILE_COUNT="$(printf '%s\n' "$CHANGED_FILES" | sed '/^$/d' | wc -l | tr -d ' ')"
FILE_COUNT="${FILE_COUNT:-0}"
STAT_TAIL="$($GIT diff --stat HEAD...upstream/main 2>/dev/null | tail -1 | sed 's/^ *//')"

# 检查官方是否修复了 Antigravity 相关 Bug 或触碰了相关文件
ANTIGRAVITY_KEYWORD_HITS="$($GIT log --oneline -i -E --grep='antigravity|service_tier|reasoning' HEAD..upstream/main 2>/dev/null || true)"
ANTIGRAVITY_FILE_HITS=""
HIT_COUNT=0

while IFS= read -r f; do
  [ -z "$f" ] && continue
  for pat in "${ANTIGRAVITY_PATTERNS[@]}"; do
    if printf '%s\n' "$f" | grep -Eq "$pat"; then
      ANTIGRAVITY_FILE_HITS="${ANTIGRAVITY_FILE_HITS}${f}"$'\n'
      HIT_COUNT=$((HIT_COUNT + 1))
      break
    fi
  done
done <<EOF
$CHANGED_FILES
EOF

# 【只提醒相关】若上游无相关关键词且未触碰相关文件，则静默退出（不打扰）
if [ -z "$ANTIGRAVITY_KEYWORD_HITS" ] && [ "$HIT_COUNT" -eq 0 ]; then
  exit 0
fi

echo "🔔 CLIProxyAPI 官方上游有相关更新！"
echo
echo "1. 当前本地 HEAD: $HEAD"
echo "2. 上游 main 最新: $UPSTREAM_HEAD"
echo "3. 上游新增 commit 数: $NEW_COUNT 个"
echo "4. 改动规模: 涉及 ${FILE_COUNT} 个文件（${STAT_TAIL}）"
echo
echo "5. 上游主要提交摘要:"
while IFS= read -r line; do
  [ -n "$line" ] && echo "   - $line"
done <<EOF
$LOG_LINES
EOF
echo

echo "6. 官方修复排查重点:"
if [ -n "$ANTIGRAVITY_KEYWORD_HITS" ]; then
  echo "   ⭐ 【疑似官方已修复】提交日志中检测到 antigravity / service_tier / reasoning 关键词！"
  while IFS= read -r kw; do
    [ -n "$kw" ] && echo "     • $kw"
  done <<EOF
$ANTIGRAVITY_KEYWORD_HITS
EOF
elif [ "$HIT_COUNT" -gt 0 ]; then
  echo "   ⚠️ 官方修改了 Antigravity 核心模块文件（触碰 ${HIT_COUNT} 个文件）："
  printf '%s' "$ANTIGRAVITY_FILE_HITS" | sed '/^$/d' | head -10 | sed 's/^/     • /'
else
  echo "   ✅ 官方本次更新未触碰 Antigravity 相关文件，也未包含相关关键词（我们的魔改不受影响）。"
fi
echo

echo "7. 评估与处置建议:"
if [ -n "$ANTIGRAVITY_KEYWORD_HITS" ]; then
  echo "   建议：官方大概率已跟进修复此问题！可联系小竹子测试切回官方原版，或合并上游验证。"
elif [ "$HIT_COUNT" -gt 0 ]; then
  echo "   建议：官方动了 Antigravity 相关文件，若要合并请让小竹子协助解决可能的代码冲突。"
else
  echo "   建议：与当前魔改无冲突，可按需安全合入上游最新特性并自动重新构建镜像。"
fi
echo
echo "（说明：本任务为定时巡检，仅比对上游并推送提醒，不会自动合并或改变当前运行容器）"
exit 0
