# PowerShell build script for File Tree Scanner
# Usage: .\build.ps1 [target]
# Targets: gui (default), console, install, clean

param(
    [string]$Target = "gui"
)

$AppName = "file-tree-scanner"
$Version = Get-Date -Format "yyyy.MM.dd"

Write-Host "Building File Tree Scanner..." -ForegroundColor Green
Write-Host "Target: $Target" -ForegroundColor Yellow

switch ($Target.ToLower()) {
    "gui" {
        Write-Host "Building GUI version (no console window)..." -ForegroundColor Cyan
        go build -ldflags="-H windowsgui -X main.version=$Version" -o "$AppName.exe" ./cmd
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✓ GUI build successful: $AppName.exe" -ForegroundColor Green
        } else {
            Write-Host "✗ GUI build failed" -ForegroundColor Red
            exit 1
        }
    }
    
    "console" {
        Write-Host "Building console version (with console window)..." -ForegroundColor Cyan
        go build -ldflags="-X main.version=$Version" -o "$AppName-console.exe" ./cmd
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✓ Console build successful: $AppName-console.exe" -ForegroundColor Green
        } else {
            Write-Host "✗ Console build failed" -ForegroundColor Red
            exit 1
        }
    }
    
    "both" {
        Write-Host "Building both GUI and console versions..." -ForegroundColor Cyan
        
        # Build GUI version
        go build -ldflags="-H windowsgui -X main.version=$Version" -o "$AppName.exe" ./cmd
        if ($LASTEXITCODE -ne 0) {
            Write-Host "✗ GUI build failed" -ForegroundColor Red
            exit 1
        }
        
        # Build console version
        go build -ldflags="-X main.version=$Version" -o "$AppName-console.exe" ./cmd
        if ($LASTEXITCODE -ne 0) {
            Write-Host "✗ Console build failed" -ForegroundColor Red
            exit 1
        }
        
        Write-Host "✓ Both builds successful:" -ForegroundColor Green
        Write-Host "  - $AppName.exe (GUI)" -ForegroundColor White
        Write-Host "  - $AppName-console.exe (Console)" -ForegroundColor White
    }
    
    "install" {
        Write-Host "Installing to GOPATH/bin..." -ForegroundColor Cyan
        go install ./cmd
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✓ Installation successful" -ForegroundColor Green
            Write-Host "Run with: file-tree-scanner" -ForegroundColor White
        } else {
            Write-Host "✗ Installation failed" -ForegroundColor Red
            exit 1
        }
    }
    
    "clean" {
        Write-Host "Cleaning build artifacts..." -ForegroundColor Cyan
        Remove-Item -Path "$AppName.exe" -ErrorAction SilentlyContinue
        Remove-Item -Path "$AppName-console.exe" -ErrorAction SilentlyContinue
        Write-Host "✓ Clean completed" -ForegroundColor Green
    }
    
    "test" {
        Write-Host "Running tests..." -ForegroundColor Cyan
        go test ./...
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✓ All tests passed" -ForegroundColor Green
        } else {
            Write-Host "✗ Tests failed" -ForegroundColor Red
            exit 1
        }
    }
    
    default {
        Write-Host "Unknown target: $Target" -ForegroundColor Red
        Write-Host "Available targets:" -ForegroundColor Yellow
        Write-Host "  gui     - Build GUI version (default)" -ForegroundColor White
        Write-Host "  console - Build console version" -ForegroundColor White
        Write-Host "  both    - Build both versions" -ForegroundColor White
        Write-Host "  install - Install to GOPATH/bin" -ForegroundColor White
        Write-Host "  clean   - Remove build artifacts" -ForegroundColor White
        Write-Host "  test    - Run tests" -ForegroundColor White
        exit 1
    }
}

Write-Host "Build script completed." -ForegroundColor Green