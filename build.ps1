# Cross-compiles static evalctl binaries for all target platforms.
$targets = @(
    @{ os = "linux";   arch = "amd64" },
    @{ os = "linux";   arch = "arm64" },
    @{ os = "darwin";  arch = "amd64" },
    @{ os = "darwin";  arch = "arm64" },
    @{ os = "windows"; arch = "amd64" },
    @{ os = "windows"; arch = "arm64" }
)

New-Item -ItemType Directory -Force -Path "dist" | Out-Null

foreach ($t in $targets) {
    $ext = if ($t.os -eq "windows") { ".exe" } else { "" }
    $out = "dist/evalctl-$($t.os)-$($t.arch)$ext"
    Write-Output "Building $out"
    $env:GOOS = $t.os
    $env:GOARCH = $t.arch
    $env:CGO_ENABLED = "0"
    go build -o $out ./cmd/evalctl
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Build failed for $($t.os)/$($t.arch)"
        exit 1
    }
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

Write-Output "All builds complete."
