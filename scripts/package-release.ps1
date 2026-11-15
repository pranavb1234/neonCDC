[CmdletBinding()]
param(
  [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$packagingSource = Join-Path $repositoryRoot "release/docker-compose"
$requiredFiles = @("docker-compose.yml", ".env.example", "README.md")
$releaseName = "CDC-Lite-Docker-Compose-$Version"
$distDirectory = Join-Path $repositoryRoot "dist"
$stagingDirectory = Join-Path $distDirectory $releaseName
$zipPath = Join-Path $distDirectory "$releaseName.zip"

foreach ($file in $requiredFiles) {
  $source = Join-Path $packagingSource $file
  if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
    throw "Required packaging file is missing: $source"
  }
}

foreach ($file in $requiredFiles) {
  & git -C $repositoryRoot ls-files --error-unmatch "release/docker-compose/$file" | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "Packaging file must be tracked by Git: release/docker-compose/$file"
  }
}

New-Item -ItemType Directory -Force -Path $distDirectory | Out-Null
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $stagingDirectory
Remove-Item -Force -ErrorAction SilentlyContinue $zipPath
New-Item -ItemType Directory -Path $stagingDirectory | Out-Null
Copy-Item -LiteralPath (Join-Path $packagingSource "docker-compose.yml") -Destination $stagingDirectory
Copy-Item -LiteralPath (Join-Path $packagingSource ".env.example") -Destination $stagingDirectory
Copy-Item -LiteralPath (Join-Path $packagingSource "README.md") -Destination $stagingDirectory

Compress-Archive -LiteralPath $stagingDirectory -DestinationPath $zipPath -CompressionLevel Optimal
$checksum = (Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath).Hash.ToLowerInvariant()

Write-Output "Release ZIP: $zipPath"
Write-Output "SHA256: $checksum"
