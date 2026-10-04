param(
    [Parameter(Mandatory=$true)]
    [ValidatePattern('^v[0-9]+\.[0-9]+\.[0-9]+-tangboys([.-][0-9A-Za-z]+)*$')]
    [string]$Version,
    [ValidateSet('linux/amd64','linux/arm64','darwin/amd64','darwin/arm64','windows/amd64','windows/arm64')]
    [string[]]$Targets = @('linux/amd64','linux/arm64','darwin/amd64','darwin/arm64','windows/amd64','windows/arm64')
)
$ErrorActionPreference='Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$output = Join-Path $root 'dist/release'
New-Item -ItemType Directory -Path $output -Force | Out-Null
$oldGOOS, $oldGOARCH, $oldCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
Push-Location -LiteralPath $root
try {
    $commit = (& git rev-parse --short HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine release commit.' }
    $buildTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    $flags = "-s -w -X main.version=$Version -X main.commit=$commit -X main.buildTime=$buildTime"
    foreach ($target in $Targets) {
        $env:GOOS, $env:GOARCH = $target.Split('/')
        $env:CGO_ENABLED = '0'
        $stem = "cc-connect-$Version-$($env:GOOS)-$($env:GOARCH)"
        $stage = Join-Path $output $stem
        New-Item -ItemType Directory -Path $stage -Force | Out-Null
        $binary = 'cc-connect'
        if ($env:GOOS -eq 'windows') { $binary += '.exe' }
        & go build -trimpath -tags goolm -ldflags $flags -o (Join-Path $stage $binary) ./cmd/cc-connect
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $target" }
        Copy-Item -LiteralPath (Join-Path $root 'config.example.toml') -Destination $stage
        Copy-Item -LiteralPath (Join-Path $root 'INSTALL.md') -Destination $stage
        Copy-Item -LiteralPath (Join-Path $root 'README.md') -Destination $stage
        Copy-Item -LiteralPath (Join-Path $root "changelogs/$Version.md") -Destination (Join-Path $stage 'RELEASE_NOTES.md')
        Copy-Item -LiteralPath (Join-Path $root 'codex-plugin') -Destination $stage -Recurse -Force
        $marketplaceDir = Join-Path $stage '.agents/plugins'
        New-Item -ItemType Directory -Path $marketplaceDir -Force | Out-Null
        Copy-Item -LiteralPath (Join-Path $root '.agents/plugins/marketplace.json') -Destination $marketplaceDir
        if ($env:GOOS -eq 'windows') {
            & go build -trimpath -ldflags '-s -w -H windowsgui' -o (Join-Path $stage 'cc-connect-plugin.exe') ./tools/windows-plugin-launcher
            if ($LASTEXITCODE -ne 0) { throw "Plugin launcher build failed: $target" }
            $shellDir = Join-Path $stage 'tools/windows-utf8-shell'
            New-Item -ItemType Directory -Path $shellDir -Force | Out-Null
            Copy-Item -LiteralPath (Join-Path $root 'tools/windows-utf8-shell/install.ps1') -Destination $shellDir
            Copy-Item -LiteralPath (Join-Path $root 'tools/windows-utf8-shell/README.md') -Destination $shellDir
            & go build -trimpath -ldflags '-s -w' -o (Join-Path $shellDir 'pwsh.exe') ./tools/windows-utf8-shell
            if ($LASTEXITCODE -ne 0) { throw "UTF-8 shell build failed: $target" }
            Add-Type -AssemblyName System.IO.Compression.FileSystem
            $archive = Join-Path $output ($stem + '.zip')
            if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
            [IO.Compression.ZipFile]::CreateFromDirectory($stage, $archive)
        } else {
            $archive = Join-Path $output ($stem + '.tar.gz')
            & tar -czf $archive -C $stage .
            if ($LASTEXITCODE -ne 0) { throw "Archive failed: $target" }
        }
        Write-Output "Built $archive"
    }
} finally {
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $oldGOOS, $oldGOARCH, $oldCGO
    Pop-Location
}
