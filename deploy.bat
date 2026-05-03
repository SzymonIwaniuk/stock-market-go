@echo off
setlocal enabledelayedexpansion

set PORT=%1
if "%PORT%"=="" set PORT=8080
set MAX_RETRIES=30
set RETRY_INTERVAL=2

echo === Stock Market Go - Deploy ===
echo Port: %PORT%
echo.

echo [1/3] Building and starting containers...
set PORT=%PORT%
docker compose up --build -d
if errorlevel 1 (
    echo ERROR: docker compose failed
    exit /b 1
)

echo [2/3] Waiting for service to be healthy...
set ATTEMPT=0

:healthloop
set /a ATTEMPT+=1
if %ATTEMPT% gtr %MAX_RETRIES% goto :healthfail

curl -sf "http://localhost:%PORT%/health" >nul 2>&1
if %errorlevel%==0 (
    echo   Health check passed ^(attempt %ATTEMPT%/%MAX_RETRIES%^)
    echo.
    echo [3/3] Service is ready!
    echo   URL: http://localhost:%PORT%
    echo.
    echo   Useful commands:
    echo     docker compose logs -f        - follow logs
    echo     docker compose down -v        - stop and clean up
    echo     make e2e-test PORT=%PORT%     - run end-to-end tests
    exit /b 0
)

echo   Waiting... ^(%ATTEMPT%/%MAX_RETRIES%^)
timeout /t %RETRY_INTERVAL% /nobreak >nul
goto :healthloop

:healthfail
echo.
echo ERROR: Service failed to become healthy after retries
echo Container logs:
docker compose logs
exit /b 1
