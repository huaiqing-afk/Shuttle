$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Push-Location $projectRoot
try {
    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    $goExecutable = if ($goCommand) { $goCommand.Source } else { Join-Path $env:ProgramFiles 'Go\bin\go.exe' }
    if (-not (Test-Path $goExecutable)) { throw 'Go is unavailable. Install Go and reopen the terminal.' }
    & $goExecutable fmt ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go formatting failed' }
    & $goExecutable vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
    & $goExecutable test ./... -count=1 -timeout=90s
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
    & $goExecutable build ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
    Push-Location frontend
    try {
        npm run typecheck
        if ($LASTEXITCODE -ne 0) { throw 'Frontend typecheck failed' }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
    } finally { Pop-Location }
    # Geometry runs against the freshly built dist, so it must come after the build.
    # Skipping it silently would let a 12px/horizontal-scroll regression through.
    $node = Get-Command node -ErrorAction SilentlyContinue
    if (-not $node) { throw 'Node is unavailable. Geometry acceptance cannot run.' }
    & $node.Source 'work/test-artifacts/geometry-check.cjs'
    if ($LASTEXITCODE -ne 0) { throw 'Geometry acceptance failed' }
} finally { Pop-Location }
