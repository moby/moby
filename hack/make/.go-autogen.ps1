<#
.NOTES
    Author:  @jhowardmsft

    Summary: Windows native version of .go-autogen which generates the
             .go source code for building, and performs resource compilation.

.PARAMETER CommitString
     The commit string. This is calculated externally to this script.

.PARAMETER DockerVersion
     The version such as 17.04.0-dev. This is calculated externally to this script.

.PARAMETER Platform
     The platform name, such as "Docker Engine - Community".

.PARAMETER Product
     The product name, used to set version.ProductName, which is used to set BuildKit's
     ExportedProduct variable in order to show useful error messages to users when a
     certain version of the product doesn't support a BuildKit feature.

.PARAMETER DefaultProductLicense
     Sets the version.DefaultProductLicense string, such as "Community Engine". This field
     can contain a summary of the product license of the daemon if a commercial license has
     been applied to the daemon.

.PARAMETER PackagerName
     The name of the packager (e.g. "Docker, Inc."). This used to set CompanyName in the manifest.
#>

param(
    [Parameter(Mandatory=$true)][string]$CommitString,
    [Parameter(Mandatory=$true)][string]$DockerVersion,
    [Parameter(Mandatory=$false)][string]$Platform,
    [Parameter(Mandatory=$false)][string]$Product,
    [Parameter(Mandatory=$false)][string]$DefaultProductLicense,
    [Parameter(Mandatory=$false)][string]$PackagerName
)

$ErrorActionPreference = "Stop"

# Utility function to get the build date/time in UTC
Function Get-BuildDateTime() {
    return $(Get-Date).ToUniversalTime()
}

Function Get-Year() {
    return $(Get-Date).year
}

Function Get-FixQuadVersionNumber($number) {
    if ($number -eq 0) {
        return $number
    }
    return $number.TrimStart("0")
}

try {
    $buildDateTime=Get-BuildDateTime
    $currentYear=Get-Year

    # Update PATH
    $env:PATH="$env:GOPATH\bin;$env:PATH"

    # Generate a version in the form major,minor,patch,build
    $versionQuad=($DockerVersion -replace "[^0-9.]*")
    if ($versionQuad -Match "^\d+`.\d+`.\d+$"){
        $versionQuad = $versionQuad + ".0"
    }
    $versionMatches = $($versionQuad | Select-String -AllMatches -Pattern "(\d+)`.(\d+)`.(\d+)`.(\d+)").Matches

    $mkwinresContents = '{
  "RT_GROUP_ICON": {
    "#1": {
      "0409": "../../winresources/docker.ico"
    }
  },
  "RT_MANIFEST": {
    "#1": {
      "0409": {
        "identity": {},
        "description": "Docker Engine",
        "minimum-os": "vista",
        "execution-level": "",
        "ui-access": false,
        "auto-elevate": false,
        "dpi-awareness": "unaware",
        "disable-theming": false,
        "disable-window-filtering": false,
        "high-resolution-scrolling-aware": false,
        "ultra-high-resolution-scrolling-aware": false,
        "long-path-aware": false,
        "printer-driver-isolation": false,
        "gdi-scaling": false,
        "segment-heap": false,
        "use-common-controls-v6": false
      }
    }
  },
  "RT_MESSAGETABLE": {
    "#1": {
      "0409": "../../winresources/event_messages.bin"
    }
  },
  "RT_VERSION": {
    "#1": {
      "0409": {
        "fixed": {
          "file_version": "'+(Get-FixQuadVersionNumber($versionMatches.Groups[1].Value))+'.'+(Get-FixQuadVersionNumber($versionMatches.Groups[2].Value))+'.'+(Get-FixQuadVersionNumber($versionMatches.Groups[3].Value))+'.'+(Get-FixQuadVersionNumber($versionMatches.Groups[4].Value))+'",
          "product_version": "'+(Get-FixQuadVersionNumber($versionMatches.Groups[1].Value))+'.'+(Get-FixQuadVersionNumber($versionMatches.Groups[2].Value))+'.'+(Get-FixQuadVersionNumber($versionMatches.Groups[3].Value))+'.'+(Get-FixQuadVersionNumber($versionMatches.Groups[4].Value))+'",
          "type": "Unknown"
        },
        "info": {
          "0000": {
            "CompanyName": "'+$PackagerName+'",
            "FileVersion": "'+$DockerVersion+'",
            "LegalCopyright": "Copyright (C) 2015-'+$currentYear+' Docker Inc.",
            "OriginalFileName": "dockerd.exe",
            "ProductName": "'+$Product+'",
            "ProductVersion": "'+$DockerVersion+'",
            "SpecialBuild": "'+$CommitString+'"
          }
        }
      }
    }
  }
}'

    # Write the file
    $outputFile="$(Get-Location)\cli\winresources\dockerd\winres.json"
    if (Test-Path $outputFile) { Remove-Item $outputFile }
    [System.IO.File]::WriteAllText($outputFile, $mkwinresContents)
    Get-Content $outputFile | Out-Host

    # Create winresources package stub if removed while using tmpfs in Dockerfile
    $stubPackage="$(Get-Location)\cli\winresources\dockerd\winresources.go"
    if(![System.IO.File]::Exists($stubPackage)){
        Set-Content -NoNewline -Path $stubPackage -Value 'package winresources'
    }

    # Generate
    go generate -v "github.com/docker/docker/cmd/dockerd"
    if ($LASTEXITCODE -ne 0) { Throw "Failed to generate version info" }
}
Catch [Exception] {
    # Throw the error onto the caller to display errors. We don't expect this script to be called directly 
    Throw ".go-autogen.ps1 failed with error $_"
}
Finally {
    $env:_ag_dockerVersion=""
    $env:_ag_gitCommit=""
}
