# Install mcpdeck on Windows (NFR-7).
#
#   irm https://github.com/abhijeet-oxide/mcpDeck/releases/latest/download/install.ps1 | iex
#
# Environment:
#   MCPDECK_VERSION         a tag such as v0.2.0 (default: the latest release)
#   MCPDECK_INSTALL_DIR     where to put mcpdeck.exe (default: %LOCALAPPDATA%\Programs\mcpdeck)
#   MCPDECK_DOWNLOAD_URL    base URL of the release files (for mirrors and tests)
#   MCPDECK_NO_MODIFY_PATH  set to 1 to leave the user PATH alone
#
# Downloads the zip for this CPU, checks it against checksums.txt, installs
# mcpdeck.exe for the current user (no admin) and adds its folder to the user
# PATH. Works in Windows PowerShell 5.1 and PowerShell 7.

& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is much faster without the progress bar
  [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

  $repo = 'abhijeet-oxide/mcpDeck'
  $version = if ($env:MCPDECK_VERSION) { $env:MCPDECK_VERSION } else { 'latest' }
  $dir = if ($env:MCPDECK_INSTALL_DIR) { $env:MCPDECK_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\mcpdeck' }

  $cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
  $arch = switch ($cpu) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "mcpdeck install: no build for CPU '$cpu'. Download one from https://github.com/$repo/releases" }
  }

  if ($env:MCPDECK_DOWNLOAD_URL) {
    $base = $env:MCPDECK_DOWNLOAD_URL.TrimEnd('/')
  } elseif ($version -eq 'latest') {
    $base = "https://github.com/$repo/releases/latest/download"
  } else {
    if (-not $version.StartsWith('v')) { $version = "v$version" }
    $base = "https://github.com/$repo/releases/download/$version"
  }

  $archive = "mcpdeck_windows_$arch.zip"
  $tmp = Join-Path ([IO.Path]::GetTempPath()) ("mcpdeck-" + [Guid]::NewGuid())
  New-Item -ItemType Directory -Path $tmp | Out-Null
  try {
    Write-Host "Downloading $base/$archive"
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$archive" -OutFile (Join-Path $tmp $archive)
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

    $want = $null
    foreach ($line in Get-Content (Join-Path $tmp 'checksums.txt')) {
      $parts = $line -split '\s+'
      if ($parts.Count -ge 2 -and $parts[1] -eq $archive) { $want = $parts[0].ToLower() }
    }
    if (-not $want) { throw "mcpdeck install: $archive is not listed in checksums.txt" }
    $got = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $tmp $archive)).Hash.ToLower()
    if ($want -ne $got) { throw "mcpdeck install: checksum mismatch for $archive (want $want, got $got); nothing was installed" }

    Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath (Join-Path $tmp 'x') -Force
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $exe = Join-Path $dir 'mcpdeck.exe'
    $running = Get-Process -Name mcpdeck -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe }
    if ($running) { throw "mcpdeck install: mcpdeck is running from $exe. Stop it (mcpdeck stop) and run the installer again." }
    Copy-Item -Force -Path (Join-Path $tmp 'x\mcpdeck.exe') -Destination $exe
    Unblock-File -Path $exe -ErrorAction SilentlyContinue
  } finally {
    Remove-Item -Recurse -Force -Path $tmp -ErrorAction SilentlyContinue
  }

  Write-Host ("Installed " + (& $exe version))

  if ($env:MCPDECK_NO_MODIFY_PATH -ne '1') {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @($userPath -split ';' | Where-Object { $_ })
    if ($entries -notcontains $dir) {
      [Environment]::SetEnvironmentVariable('Path', (($entries + $dir) -join ';'), 'User')
      Write-Host "Added $dir to your user PATH. New terminals will find mcpdeck."
    }
    if (($env:Path -split ';') -notcontains $dir) { $env:Path = "$env:Path;$dir" }
  }
  Write-Host 'Run: mcpdeck'
}
