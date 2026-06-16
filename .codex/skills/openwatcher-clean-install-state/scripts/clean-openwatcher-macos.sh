#!/usr/bin/env bash
set -euo pipefail

YES=0
RESTORE=""
LIST_BACKUPS=0
BACKUP_ROOT="$HOME/OpenWatcherBackups/openwatcher-config"
CONFIG_DIR="$HOME/.openwatcher"

usage() {
  cat <<'USAGE'
用法：clean-openwatcher-macos.sh [选项]

默认只预览，不删除文件。

选项：
  --yes                 执行清理或恢复
  --restore VALUE       恢复备份；VALUE 可为 latest 或时间戳
  --list-backups        列出可恢复的 ~/.openwatcher 备份
  --backup-root DIR     指定备份根目录
  --help                显示帮助
USAGE
}

log() {
  printf '[openwatcher-clean] %s\n' "$*"
}

die() {
  printf '[openwatcher-clean] 错误：%s\n' "$*" >&2
  exit 1
}

timestamp() {
  date '+%Y%m%d-%H%M%S'
}

backup_dirs() {
  [[ -d "$BACKUP_ROOT" ]] || return 0
  find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name '[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9][0-9][0-9]' 2>/dev/null | sort
}

latest_backup() {
  backup_dirs | tail -n 1
}

write_backup_info() {
  local dest="$1"
  {
    printf 'created_at=%s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    printf 'platform=macos\n'
    printf 'source=%s\n' "${CONFIG_DIR}"
    printf 'backup=%s\n' "$dest/.openwatcher"
  } > "$dest/backup-info.txt"
}

backup_config() {
  if [[ ! -d "${CONFIG_DIR}" ]]; then
    log "未发现 ${CONFIG_DIR}，无需备份配置"
    return 0
  fi
  local dest="$BACKUP_ROOT/$(timestamp)"
  if [[ "$YES" != "1" ]]; then
    log "将备份 ${CONFIG_DIR} 到 $dest/.openwatcher"
    return 0
  fi
  mkdir -p "$dest"
  cp -a "${CONFIG_DIR}" "$dest/.openwatcher"
  write_backup_info "$dest"
  log "已备份 ${CONFIG_DIR} 到 $dest/.openwatcher"
}

stop_processes() {
  local patterns=(
    '/Applications/OpenWatcher.app'
    "$HOME/Applications/OpenWatcher.app"
    "$HOME/Library/Application Support/OpenWatcher"
  )
  for pattern in "${patterns[@]}"; do
    if pgrep -f "$pattern" >/dev/null 2>&1; then
      if [[ "$YES" == "1" ]]; then
        pkill -f "$pattern" || true
        log "已停止匹配进程：$pattern"
      else
        log "将停止匹配进程：$pattern"
      fi
    fi
  done
}

clean_state() {
  local targets=(
    "/Applications/OpenWatcher.app"
    "$HOME/Applications/OpenWatcher.app"
    "$HOME/Library/Application Support/OpenWatcher"
    "$HOME/Library/WebKit/ai.openwatcher.desktop"
    "$HOME/Library/Caches/ai.openwatcher.desktop"
    "$HOME/Library/Caches/OpenWatcher"
    "$HOME/Library/Logs/OpenWatcher"
    "$HOME/Library/Preferences/ai.openwatcher.desktop.plist"
    "${CONFIG_DIR}"
  )

  stop_processes
  backup_config

  for target in "${targets[@]}"; do
    if [[ -e "$target" ]]; then
      if [[ "$YES" == "1" ]]; then
        rm -rf "$target"
        log "已清理 $target"
      else
        log "将清理 $target"
      fi
    else
      log "不存在，跳过 $target"
    fi
  done

  if [[ "$YES" != "1" ]]; then
    log "当前为预览模式；确认后追加 --yes 执行清理"
  fi
}

resolve_backup() {
  local value="$1"
  local selected=""
  if [[ "$value" == "latest" ]]; then
    selected="$(latest_backup)"
  else
    selected="$BACKUP_ROOT/$value"
  fi
  [[ -n "$selected" && -d "$selected/.openwatcher" ]] || die "找不到可恢复备份：$value"
  printf '%s\n' "$selected"
}

restore_config() {
  local selected
  selected="$(resolve_backup "$RESTORE")"
  local pre_restore="$BACKUP_ROOT/pre-restore-$(timestamp)"

  log "恢复来源：$selected/.openwatcher"
  log "恢复目标：${CONFIG_DIR}"
  if [[ -d "${CONFIG_DIR}" ]]; then
    log "当前配置将先移动到：$pre_restore/.openwatcher"
  fi

  if [[ "$YES" != "1" ]]; then
    log "当前为预览模式；确认后追加 --yes 执行恢复"
    return 0
  fi

  if [[ -d "${CONFIG_DIR}" ]]; then
    mkdir -p "$pre_restore"
    mv "${CONFIG_DIR}" "$pre_restore/.openwatcher"
    write_backup_info "$pre_restore"
    log "已保存恢复前配置到 $pre_restore/.openwatcher"
  fi
  mkdir -p "$(dirname "${CONFIG_DIR}")"
  cp -a "$selected/.openwatcher" "${CONFIG_DIR}"
  log "已恢复配置到 ${CONFIG_DIR}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes)
      YES=1
      shift
      ;;
    --restore)
      RESTORE="${2:-}"
      [[ -n "$RESTORE" ]] || die "--restore 需要 latest 或时间戳"
      shift 2
      ;;
    --list-backups)
      LIST_BACKUPS=1
      shift
      ;;
    --backup-root)
      BACKUP_ROOT="${2:-}"
      [[ -n "$BACKUP_ROOT" ]] || die "--backup-root 需要目录"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "未知参数：$1"
      ;;
  esac
done

[[ "$(uname -s)" == "Darwin" ]] || die "本脚本只能在 macOS 上运行"

if [[ "$LIST_BACKUPS" == "1" ]]; then
  backup_dirs | while IFS= read -r dir; do
    printf '%s\n' "$(basename "$dir")"
  done
  exit 0
fi

if [[ -n "$RESTORE" ]]; then
  restore_config
else
  clean_state
fi
