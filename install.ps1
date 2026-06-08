# Code-Nimai Installer for Windows
# Usage: iex (iwr https://raw.githubusercontent.com/YOUR_USER/YOUR_REPO/main/install.ps1)

$installDir = "$HOME\.code-nimai"
$binDir = "$installDir\bin"
$exeName = "code-nimai.exe"
$exePath = "$binDir\$exeName"

# Create directories
if (!(Test-Path $binDir)) {
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
}

Write-Host "🚀 Installing Code-Nimai..." -ForegroundColor Cyan

# Download the latest binary (Replace with your actual GitHub Release URL)
# For now, we assume the binary is uploaded to your repo
$downloadUrl = "https://github.com/YOUR_USER/YOUR_REPO/releases/latest/download/mcp-codebase-server.exe"

try {
    Write-Host "📥 Downloading binary..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $exePath -ErrorAction Stop
} catch {
    Write-Host "❌ Failed to download binary. Please ensure the URL is correct and public." -ForegroundColor Red
    return
}

# Add to Path for current session
if ($env:Path -notlike "*$binDir*") {
    $env:Path += ";$binDir"
    
    # Add to Path permanently for User
    $oldPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($oldPath -notlike "*$binDir*") {
        [Environment]::SetEnvironmentVariable("Path", $oldPath + ";$binDir", "User")
        Write-Host "✅ Added $binDir to your User PATH." -ForegroundColor Green
    }
}

Write-Host "`n✨ Code-Nimai installed successfully!" -ForegroundColor Green
Write-Host "💡 You can now run it from any directory using the command: " -NoNewline
Write-Host "code-nimai" -ForegroundColor Yellow
Write-Host "🔄 Please restart your terminal to apply PATH changes." -ForegroundColor Gray
