$ErrorActionPreference = 'Stop'

$architecture = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower()
$arch = switch ($architecture) {
    x64   { 'amd64' }
    arm64 { 'arm64' }
    x86   { '386' }
    default { throw "Unsupported CPU architecture: $architecture" }
}

$release = Invoke-RestMethod 'https://api.github.com/repos/rahulsom/dogu/releases/latest'
$asset = $release.assets | Where-Object name -like "*_windows_$arch.zip" | Select-Object -First 1
if (-not $asset) {
    throw "No release asset found for windows/$arch"
}
$checksums = $release.assets | Where-Object name -eq 'checksums.txt' | Select-Object -First 1
if (-not $checksums) {
    throw 'Release does not include checksums.txt'
}

$doguHome = if ($env:DOGU_HOME) { $env:DOGU_HOME } else { Join-Path $HOME '.dogu' }
$installDir = if ($env:DOGU_INSTALL_DIR) { $env:DOGU_INSTALL_DIR } else { Join-Path $doguHome 'bin' }
$tempDir = Join-Path ([IO.Path]::GetTempPath()) "dogu-$([guid]::NewGuid())"
New-Item -ItemType Directory -Force $installDir, $tempDir | Out-Null
try {
    $archive = Join-Path $tempDir 'dogu.zip'
    Invoke-WebRequest $asset.browser_download_url -OutFile $archive
    $checksumsFile = Join-Path $tempDir 'checksums.txt'
    Invoke-WebRequest $checksums.browser_download_url -OutFile $checksumsFile

    $expectedHash = $null
    foreach ($line in Get-Content $checksumsFile) {
        if ($line -match '^([0-9a-fA-F]{64})\s+\*?(.+)$' -and $Matches[2] -ceq $asset.name) {
            $expectedHash = $Matches[1]
            break
        }
    }
    if (-not $expectedHash) {
        throw "No valid checksum found for $($asset.name)"
    }
    $actualHash = (Get-FileHash $archive -Algorithm SHA256).Hash
    if ($actualHash -ine $expectedHash) {
        throw "SHA-256 checksum verification failed for $($asset.name)"
    }

    Expand-Archive $archive -DestinationPath $tempDir
    Move-Item (Join-Path $tempDir 'dogu.exe') (Join-Path $installDir 'dogu.exe') -Force
}
finally {
    Remove-Item $tempDir -Recurse -Force
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$pathEntries = @($userPath -split ';' | Where-Object { $_ })
if ($pathEntries -notcontains $installDir) {
    $newPath = (@($installDir) + $pathEntries) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
}
$env:Path = "$installDir;$env:Path"
dogu --help
