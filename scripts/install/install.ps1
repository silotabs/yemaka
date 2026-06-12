param(
  [ValidateSet("web", "cli")]
  [string]$Type = "",
  [string]$Home = "",
  [string]$BinDir = "",
  [switch]$Yes,
  [switch]$NoShim,
  [switch]$SkipFrontendBuild,
  [int]$Port = 7727
)

$ErrorActionPreference = "Stop"

function Ask([string]$Prompt, [string]$Default) {
  if ($Yes) { return $Default }
  $answer = Read-Host "$Prompt [$Default]"
  if ([string]::IsNullOrWhiteSpace($answer)) { return $Default }
  return $answer
}

function Confirm([string]$Prompt, [string]$Default) {
  $answer = (Ask $Prompt $Default).ToLowerInvariant()
  return @("y", "yes", "true", "1").Contains($answer)
}

function Command-Exists([string]$Name) {
  return $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Default-Home {
  if ($env:APPDATA) { return (Join-Path $env:APPDATA "Yemaka") }
  return (Join-Path $HOME "AppData\Roaming\Yemaka")
}

function Default-BinDir {
  if ($env:LOCALAPPDATA) { return (Join-Path $env:LOCALAPPDATA "Programs\Yemaka") }
  return (Join-Path $HOME "AppData\Local\Programs\Yemaka")
}

function Normalize-PathString([string]$PathValue) {
  $full = [System.IO.Path]::GetFullPath($PathValue)
  $root = [System.IO.Path]::GetPathRoot($full)
  if ($full.Length -gt $root.Length) {
    $full = $full.TrimEnd([char[]]@('\','/'))
  }
  return $full
}

function Path-IsEqualOrInside([string]$Parent, [string]$Child) {
  $parentPath = Normalize-PathString $Parent
  $childPath = Normalize-PathString $Child
  if ($childPath.Equals($parentPath, [System.StringComparison]::OrdinalIgnoreCase)) { return $true }
  $separator = [System.IO.Path]::DirectorySeparatorChar
  return $childPath.StartsWith("$parentPath$separator", [System.StringComparison]::OrdinalIgnoreCase)
}

function Assert-SafeInstallHome([string]$PathValue, [string]$RepoRootValue) {
  $path = Normalize-PathString $PathValue
  $homePath = Normalize-PathString $HOME
  $root = [System.IO.Path]::GetPathRoot($path)
  if ([string]::IsNullOrWhiteSpace($path) -or $path.Equals($root, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing unsafe Yemaka home: $path"
  }
  if ($path.Equals($homePath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to install into your home directory: $path"
  }
  if (Path-IsEqualOrInside $path $RepoRootValue) {
    throw "Refusing install home that contains the repository: $path"
  }
  if (Path-IsEqualOrInside $RepoRootValue $path) {
    throw "Refusing install home inside the source repository: $path"
  }
  $protected = @(
    (Join-Path $homePath "Desktop"),
    (Join-Path $homePath "Documents"),
    (Join-Path $homePath "Downloads"),
    (Join-Path $homePath "AppData"),
    (Join-Path $homePath "AppData\Roaming"),
    (Join-Path $homePath "AppData\Local")
  )
  foreach ($item in $protected) {
    if ($path.Equals((Normalize-PathString $item), [System.StringComparison]::OrdinalIgnoreCase)) {
      throw "Refusing common user directory as Yemaka home: $path"
    }
  }
  if (-not ([System.IO.Path]::GetFileName($path).ToLowerInvariant().Contains("yemaka"))) {
    throw "Yemaka home directory name must include 'yemaka': $path"
  }
}

function Assert-SafeBinDir([string]$PathValue, [string]$RepoRootValue) {
  $path = Normalize-PathString $PathValue
  $homePath = Normalize-PathString $HOME
  $root = [System.IO.Path]::GetPathRoot($path)
  if ([string]::IsNullOrWhiteSpace($path) -or $path.Equals($root, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing unsafe command directory: $path"
  }
  if ($path.Equals($homePath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing command directory equal to your home directory: $path"
  }
  if (Path-IsEqualOrInside $path $RepoRootValue) {
    throw "Refusing command directory that contains the repository: $path"
  }
  if (Path-IsEqualOrInside $RepoRootValue $path) {
    throw "Refusing command directory inside the source repository: $path"
  }
}

function Add-UserPath([string]$PathToAdd) {
  $current = [Environment]::GetEnvironmentVariable("Path", "User")
  $parts = @()
  if ($current) { $parts = $current.Split(";") | Where-Object { $_ } }
  if ($parts -contains $PathToAdd) { return $false }
  $updated = (($parts + $PathToAdd) -join ";")
  [Environment]::SetEnvironmentVariable("Path", $updated, "User")
  return $true
}

if ([string]::IsNullOrWhiteSpace($Type)) {
  $Type = Ask "Choose installation type: web (Web + CLI/TUI) or cli (CLI/TUI only)" "web"
}
if ($Type -ne "web" -and $Type -ne "cli") {
  throw "Installation type must be web or cli."
}
if ([string]::IsNullOrWhiteSpace($Home)) {
  $Home = Ask "Choose Yemaka data directory" (Default-Home)
}
if ([string]::IsNullOrWhiteSpace($BinDir)) {
  $BinDir = Ask "Choose command directory" (Default-BinDir)
}

$RepoRoot = Normalize-PathString (Resolve-Path (Join-Path $PSScriptRoot "..\.."))
$InstallHome = Normalize-PathString $Home
$BinDir = Normalize-PathString $BinDir
Assert-SafeInstallHome $InstallHome $RepoRoot
Assert-SafeBinDir $BinDir $RepoRoot
$InstallBinDir = Join-Path $InstallHome "libexec"
$InstallBin = Join-Path $InstallBinDir "yemaka.exe"
$LogDir = Join-Path $InstallHome "logs"
New-Item -ItemType Directory -Force -Path $InstallHome, $InstallBinDir, $LogDir | Out-Null
@"
Yemaka install home
created_at=$((Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ"))
repo=$RepoRoot
"@ | Set-Content -Encoding ASCII (Join-Path $InstallHome ".yemaka-install-root")
$LogFile = Join-Path $LogDir ("install-{0:yyyyMMddTHHmmssZ}.log" -f (Get-Date).ToUniversalTime())

Write-Host "Yemaka public-beta installer"
Write-Host "Repository: $RepoRoot"
Write-Host "Install type: $Type"
Write-Host "Yemaka home: $InstallHome"
Write-Host "Command directory: $BinDir"
Write-Host "Log: $LogFile"

$os = Get-CimInstance Win32_OperatingSystem
$arch = (Get-CimInstance Win32_Processor | Select-Object -First 1).Architecture
$ramGB = [math]::Round($os.TotalVisibleMemorySize / 1MB, 1)
$drive = Get-PSDrive -Name ([System.IO.Path]::GetPathRoot($InstallHome).Substring(0,1))
$diskGB = [math]::Round($drive.Free / 1GB, 1)
Write-Host "System: Windows arch=$arch"
Write-Host "RAM: $ramGB GB"
Write-Host "Disk near install home: $diskGB GB available"

$portUsed = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if ($portUsed) {
  Write-Warning "Port $Port is already in use. You can run yemaka serve --addr 127.0.0.1:<free-port>."
} else {
  Write-Host "Port $Port: available"
}

if (Command-Exists "ollama") {
  Write-Host "Ollama: found"
  try {
    & ollama list *> $LogFile
    Write-Host "Ollama model list: ok"
  } catch {
    Write-Warning "Ollama is installed but not responding. Start it before first chat."
  }
} else {
  Write-Warning "Ollama was not found. Install Ollama and a small local model before first chat."
}

Write-Host ""
Write-Host "Model guidance:"
Write-Host "  4GB RAM: CLI/TUI low-memory only; use a small 2B quantized model."
Write-Host "  8GB RAM: recommended minimum for web + a small local model."
Write-Host "  16GB+ RAM: stronger 4B local models become more comfortable."
Write-Host "  Cross-platform low-memory option: qwen3.5:2b-q4_K_M if available."

$configureModel = Confirm "Do you want to configure Ollama/model settings now?" "no"
$modelName = ""
if ($configureModel) {
  $modelName = Ask "Installed model name to use for default and low-memory roles" ""
}
$lowMemory = Ask "Do you want to use low-memory mode guidance?" "yes"
$disabledDefaults = Ask "Keep internet/search/cloud/connectors/embeddings disabled by default?" "yes"
if (-not @("yes","y","true","1").Contains($disabledDefaults.ToLowerInvariant())) {
  Write-Warning "Installer will still preserve safe disabled defaults. Enable optional systems later from settings/CLI."
}

Set-Location $RepoRoot
if (Test-Path (Join-Path $RepoRoot "cmd\yemaka\main.go")) {
  if (-not (Command-Exists "go")) { throw "Go is required to build Yemaka from source." }
  Write-Host "Building Yemaka CLI/TUI/server binary..."
  & go build -o $InstallBin .\cmd\yemaka *> $LogFile
} elseif (Command-Exists "yemaka") {
  Copy-Item (Get-Command yemaka).Source $InstallBin -Force
} else {
  throw "Cannot find source entrypoint or an existing yemaka binary."
}

if ($Type -eq "web") {
  $distIndex = Join-Path $RepoRoot "frontend\dist\index.html"
  if (-not (Test-Path $distIndex)) {
    if ($SkipFrontendBuild) { throw "frontend\dist is missing and frontend build was skipped." }
    if (-not (Command-Exists "npm")) { throw "npm is required because frontend\dist is missing." }
    Write-Host "Building web frontend..."
    & npm --prefix frontend run build *> $LogFile
  }
  $webDir = Join-Path $InstallHome "web"
  $webDist = Join-Path $webDir "dist"
  if (Test-Path $webDist) { Remove-Item -Recurse -Force $webDist }
  New-Item -ItemType Directory -Force -Path $webDir | Out-Null
  Copy-Item -Recurse (Join-Path $RepoRoot "frontend\dist") $webDist
  Write-Host "Installed web assets: $webDist"
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
$shim = Join-Path $BinDir "yemaka.cmd"
if (-not $NoShim) {
  @"
@echo off
set "YEMAKA_HOME=$InstallHome"
"$InstallBin" %*
"@ | Set-Content -Encoding ASCII $shim
  Write-Host "Command shim created: $shim"
  if (Confirm "Add command directory to your user PATH?" "yes") {
    if (Add-UserPath $BinDir) {
      Write-Host "Added to user PATH. Open a new terminal to use 'yemaka' directly."
    } else {
      Write-Host "Command directory is already on the user PATH."
    }
  }
}

Write-Host "Creating/verifying default config..."
$env:YEMAKA_HOME = $InstallHome
& $InstallBin --help *> $LogFile
try {
  & $InstallBin doctor *>> $LogFile
  Write-Host "Doctor: completed"
} catch {
  Write-Warning "Doctor completed with warnings; see $LogFile"
}

if (-not [string]::IsNullOrWhiteSpace($modelName)) {
  Write-Host "Applying selected model roles: $modelName"
  try { & $InstallBin model set default $modelName *>> $LogFile } catch { Write-Warning "Could not set default model." }
  try { & $InstallBin model set low_memory $modelName *>> $LogFile } catch { Write-Warning "Could not set low-memory model." }
}

Write-Host ""
Write-Host "Installed. Try:"
if ($NoShim) {
  Write-Host "  `$env:YEMAKA_HOME='$InstallHome'; & '$InstallBin' --help"
  if ($Type -eq "web") { Write-Host "  `$env:YEMAKA_HOME='$InstallHome'; & '$InstallBin' serve" }
  Write-Host "  `$env:YEMAKA_HOME='$InstallHome'; & '$InstallBin' tui"
} else {
  Write-Host "  $shim --help"
  Write-Host "  $shim doctor"
  if ($Type -eq "web") { Write-Host "  $shim serve" }
  Write-Host "  $shim tui"
}
Write-Host "No cloud, internet/search, connectors, embeddings, background jobs, or model downloads were enabled by this installer."
