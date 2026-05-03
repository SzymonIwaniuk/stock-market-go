param(
    [string]$Port = "8080"
)

Set-Location (Split-Path -Parent $PSScriptRoot)

$MaxRetries = 30
$RetryInterval = 2

Write-Host "=== Stock Market Go - Deploy ===" -ForegroundColor Cyan
Write-Host "Port: $Port"
Write-Host ""

Write-Host "[1/3] Building and starting containers..." -ForegroundColor Yellow
$env:PORT = $Port
docker compose up --build -d
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: docker compose failed" -ForegroundColor Red
    exit 1
}

Write-Host "[2/3] Waiting for service to be healthy..." -ForegroundColor Yellow
for ($i = 1; $i -le $MaxRetries; $i++) {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:$Port/health" -UseBasicParsing -TimeoutSec 2
        if ($response.StatusCode -eq 200) {
            Write-Host "  Health check passed (attempt $i/$MaxRetries)" -ForegroundColor Green
            Write-Host ""
            Write-Host "[3/3] Service is ready!" -ForegroundColor Green
            Write-Host "  URL: http://localhost:$Port"
            Write-Host ""
            Write-Host "  Useful commands:"
            Write-Host "    docker compose logs -f        - follow logs"
            Write-Host "    docker compose down -v        - stop and clean up"
            Write-Host "    make e2e-test PORT=$Port      - run end-to-end tests"
            exit 0
        }
    }
    catch {
        Write-Host "  Waiting... ($i/$MaxRetries)"
    }
    Start-Sleep -Seconds $RetryInterval
}

Write-Host ""
Write-Host "ERROR: Service failed to become healthy after $($MaxRetries * $RetryInterval)s" -ForegroundColor Red
Write-Host "Container logs:"
docker compose logs
exit 1
