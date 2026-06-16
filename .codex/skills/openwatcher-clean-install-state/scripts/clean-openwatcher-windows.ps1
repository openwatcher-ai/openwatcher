param(
    [switch]$Yes,
    [string]$Restore = "",
    [switch]$ListBackups,
    [string]$BackupRoot = "$HOME\OpenWatcherBackups\openwatcher-config",
    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Usage {
    @"
用法：clean-openwatcher-windows.ps1 [选项]

默认只预览，不删除文件。

选项：
  -Yes                 执行清理或恢复
  -Restore <VALUE>     恢复备份；VALUE 可为 latest 或时间戳
  -ListBackups         列出可恢复的 ~/.openwatcher 备份
  -BackupRoot <DIR>    指定备份根目录
  -Help                显示帮助
"@
}

function Write-Step {
    param([string]$Message)
    Write-Host "[openwatcher-clean] $Message"
}

function Fail {
    param([string]$Message)
    throw "[openwatcher-clean] 错误：$Message"
}

function New-Timestamp {
    Get-Date -Format "yyyyMMdd-HHmmss"
}

function Get-ConfigDir {
    Join-Path $HOME ".openwatcher"
}

function Get-BackupDirs {
    if (-not (Test-Path $BackupRoot)) {
        return @()
    }
    Get-ChildItem -Path $BackupRoot -Directory |
        Where-Object { $_.Name -match '^\d{8}-\d{6}$' } |
        Sort-Object Name
}

function Write-BackupInfo {
    param([string]$Dest)
    $configDir = Get-ConfigDir
    @(
        "created_at=$((Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ'))"
        "platform=windows"
        "source=$configDir"
        "backup=$(Join-Path $Dest '.openwatcher')"
    ) | Set-Content -Path (Join-Path $Dest "backup-info.txt") -Encoding UTF8
}

function Backup-Config {
    $configDir = Get-ConfigDir
    if (-not (Test-Path $configDir)) {
        Write-Step "未发现 $configDir，无需备份配置"
        return
    }
    $dest = Join-Path $BackupRoot (New-Timestamp)
    $backupPath = Join-Path $dest ".openwatcher"
    if (-not $Yes) {
        Write-Step "将备份 $configDir 到 $backupPath"
        return
    }
    New-Item -ItemType Directory -Path $dest -Force | Out-Null
    Copy-Item -Path $configDir -Destination $backupPath -Recurse -Force
    Write-BackupInfo -Dest $dest
    Write-Step "已备份 $configDir 到 $backupPath"
}

function Stop-OpenWatcherProcesses {
    $patterns = @()
    if ($env:LOCALAPPDATA) {
        $patterns += (Join-Path $env:LOCALAPPDATA "OpenWatcher")
    }
    if ($env:APPDATA) {
        $patterns += (Join-Path $env:APPDATA "OpenWatcher")
    }
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $cmd = [string]$_.CommandLine
        $patterns | Where-Object { $cmd -and $cmd -like "*$_*" }
    }
    foreach ($process in $processes) {
        if ($Yes) {
            Stop-Process -Id $process.ProcessId -Force -ErrorAction SilentlyContinue
            Write-Step "已停止进程：$($process.ProcessId)"
        } else {
            Write-Step "将停止进程：$($process.ProcessId) $($process.Name)"
        }
    }
}

function Clean-State {
    $configDir = Get-ConfigDir
    $targets = @()
    if ($env:LOCALAPPDATA) {
        $targets += (Join-Path $env:LOCALAPPDATA "OpenWatcher")
    }
    if ($env:APPDATA) {
        $targets += (Join-Path $env:APPDATA "OpenWatcher")
    }
    $targets += $configDir
    $desktopPath = [Environment]::GetFolderPath("Desktop")
    if ($desktopPath) {
        $targets += (Join-Path $desktopPath "OpenWatcher.lnk")
    }

    Stop-OpenWatcherProcesses
    Backup-Config

    foreach ($target in $targets) {
        if (Test-Path $target) {
            if ($Yes) {
                Remove-Item -Path $target -Recurse -Force
                Write-Step "已清理 $target"
            } else {
                Write-Step "将清理 $target"
            }
        } else {
            Write-Step "不存在，跳过 $target"
        }
    }

    if (-not $Yes) {
        Write-Step "当前为预览模式；确认后追加 -Yes 执行清理"
    }
}

function Resolve-Backup {
    param([string]$Value)
    $selected = $null
    if ($Value -eq "latest") {
        $selected = Get-BackupDirs | Select-Object -Last 1
    } else {
        $candidate = Join-Path $BackupRoot $Value
        if (Test-Path $candidate) {
            $selected = Get-Item $candidate
        }
    }
    if (-not $selected) {
        Fail "找不到可恢复备份：$Value"
    }
    $backupConfig = Join-Path $selected.FullName ".openwatcher"
    if (-not (Test-Path $backupConfig)) {
        Fail "备份缺少 .openwatcher：$($selected.FullName)"
    }
    return $selected.FullName
}

function Restore-Config {
    $selected = Resolve-Backup -Value $Restore
    $configDir = Get-ConfigDir
    $preRestore = Join-Path $BackupRoot ("pre-restore-" + (New-Timestamp))
    $source = Join-Path $selected ".openwatcher"

    Write-Step "恢复来源：$source"
    Write-Step "恢复目标：$configDir"
    if (Test-Path $configDir) {
        Write-Step "当前配置将先移动到：$(Join-Path $preRestore '.openwatcher')"
    }

    if (-not $Yes) {
        Write-Step "当前为预览模式；确认后追加 -Yes 执行恢复"
        return
    }

    if (Test-Path $configDir) {
        New-Item -ItemType Directory -Path $preRestore -Force | Out-Null
        Move-Item -Path $configDir -Destination (Join-Path $preRestore '.openwatcher')
        Write-BackupInfo -Dest $preRestore
        Write-Step "已保存恢复前配置到 $(Join-Path $preRestore '.openwatcher')"
    }
    New-Item -ItemType Directory -Path (Split-Path $configDir -Parent) -Force | Out-Null
    Copy-Item -Path $source -Destination $configDir -Recurse -Force
    Write-Step "已恢复配置到 $configDir"
}

if ($Help) {
    Show-Usage
    exit 0
}

if ($ListBackups) {
    Get-BackupDirs | ForEach-Object { $_.Name }
    exit 0
}

if ($Restore) {
    Restore-Config
} else {
    Clean-State
}
