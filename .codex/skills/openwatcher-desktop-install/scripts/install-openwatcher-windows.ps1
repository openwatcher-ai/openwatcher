param(
    [string]$ChannelUrl = "https://openwatcher.ai/channels/beta.json",
    [string]$InstallDir = "$env:LOCALAPPDATA\OpenWatcher",
    [switch]$DryRun,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Usage {
    @"
用法：install-openwatcher-windows.ps1 [选项]

选项：
  -ChannelUrl <URL>   使用指定 OpenWatcher channel manifest
  -InstallDir <DIR>   安装目录，默认 %LOCALAPPDATA%\OpenWatcher
  -DryRun             只解析和打印计划，不下载或安装
  -Help               显示帮助
"@
}

function Write-Step {
    param([string]$Message)
    Write-Host "[openwatcher-install] $Message"
}

function Fail {
    param([string]$Message)
    throw "[openwatcher-install] 错误：$Message"
}

if ($Help) {
    Show-Usage
    exit 0
}

if (-not $IsWindows -and $PSVersionTable.PSVersion.Major -ge 6) {
    Fail "本脚本只能在 Windows 上运行"
}

if (-not $env:LOCALAPPDATA) {
    Fail "无法读取 LOCALAPPDATA"
}

$arch = $env:PROCESSOR_ARCHITECTURE
switch ($arch) {
    "ARM64" { $preferredPlatforms = @("windows-arm64", "windows-amd64") }
    "AMD64" { $preferredPlatforms = @("windows-amd64") }
    default { Fail "不支持的 Windows 架构：$arch" }
}

$workDir = Join-Path ([System.IO.Path]::GetTempPath()) ("openwatcher-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $workDir | Out-Null

try {
    Write-Step "读取 channel manifest：$ChannelUrl"
    $manifest = Invoke-RestMethod -Uri $ChannelUrl -UseBasicParsing
    if (-not $manifest.desktop -or -not $manifest.desktop.platforms) {
        Fail "manifest 缺少 desktop.platforms"
    }

    $selectedPlatform = $null
    $asset = $null
    foreach ($candidate in $preferredPlatforms) {
        $candidateAsset = $manifest.desktop.platforms.PSObject.Properties[$candidate]
        if ($candidateAsset) {
            $selectedPlatform = $candidate
            $asset = $candidateAsset.Value
            break
        }
    }

    if (-not $asset) {
        $available = ($manifest.desktop.platforms.PSObject.Properties.Name | Sort-Object) -join ", "
        if (-not $available) {
            $available = "无"
        }
        Fail "manifest 缺少当前 Windows 平台资产；可用平台：$available"
    }

    if (-not $asset.downloadUrl) {
        Fail "平台 $selectedPlatform 缺少 downloadUrl"
    }
    if (-not $asset.sha256) {
        Fail "平台 $selectedPlatform 缺少 sha256"
    }

    $artifact = $asset.artifact
    if (-not $artifact) {
        $artifact = Split-Path ([System.Uri]$asset.downloadUrl).AbsolutePath -Leaf
    }
    if (-not $artifact) {
        Fail "无法确定安装包文件名"
    }

    $desktopVersion = $manifest.desktop.version
    $releaseTag = $manifest.release.tag
    $exePath = Join-Path $InstallDir "openwatcher.exe"
    $shortcutPath = Join-Path ([Environment]::GetFolderPath("Desktop")) "OpenWatcher.lnk"

    Write-Step "平台：$selectedPlatform"
    Write-Step "Desktop 版本：$desktopVersion"
    Write-Step "Release：$releaseTag"
    Write-Step "安装包：$artifact"
    Write-Step "目标路径：$exePath"
    Write-Step "快捷方式：$shortcutPath"

    if ($DryRun) {
        Write-Step "dry-run：不会下载、安装或启动应用"
        exit 0
    }

    $downloadPath = Join-Path $workDir $artifact
    Write-Step "下载安装包"
    Invoke-WebRequest -Uri $asset.downloadUrl -OutFile $downloadPath -UseBasicParsing

    $actualSha = (Get-FileHash -Algorithm SHA256 -Path $downloadPath).Hash.ToLowerInvariant()
    $expectedSha = ([string]$asset.sha256).ToLowerInvariant()
    if ($actualSha -ne $expectedSha) {
        Fail "SHA-256 校验失败：期望 $expectedSha，实际 $actualSha"
    }
    Write-Step "SHA-256 校验通过"

    if ($artifact.EndsWith(".exe", [System.StringComparison]::OrdinalIgnoreCase)) {
        Write-Step "静默运行安装器"
        $process = Start-Process -FilePath $downloadPath -ArgumentList "/S" -Wait -PassThru
        if ($process.ExitCode -ne 0) {
            Fail "安装器退出码异常：$($process.ExitCode)"
        }
    } elseif ($artifact.EndsWith(".zip", [System.StringComparison]::OrdinalIgnoreCase)) {
        Write-Step "解压绿色包"
        if (Test-Path $InstallDir) {
            Remove-Item -Recurse -Force $InstallDir
        }
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
        Expand-Archive -Path $downloadPath -DestinationPath $InstallDir -Force
    } else {
        Fail "不支持的 Windows 安装包类型：$artifact"
    }

    if (-not (Test-Path $exePath)) {
        Fail "安装后未找到 openwatcher.exe：$exePath"
    }

    Write-Step "创建桌面快捷方式"
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = $exePath
    $shortcut.WorkingDirectory = $InstallDir
    $shortcut.IconLocation = $exePath
    $shortcut.Description = "OpenWatcher Desktop"
    $shortcut.Save()

    Write-Step "启动 OpenWatcher Desktop"
    Start-Process -FilePath $exePath
    Write-Step "安装完成"
} finally {
    if (Test-Path $workDir) {
        Remove-Item -Recurse -Force $workDir -ErrorAction SilentlyContinue
    }
}
