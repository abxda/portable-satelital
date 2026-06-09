# =============================================================================
# build_portable.ps1 - construye el Python portable (Windows x64) de
# PortableSatelital. CORRE EN WINDOWS (no cross-buildea: wheels binarias por SO).
# (ASCII puro a proposito: PowerShell 5.1 trata los caracteres tipograficos como
#  comillas y rompe el parseo; no metas acentos ni guiones largos en este .ps1.)
#
# Que hace:
#   1) baja python-build-standalone (CPython portable, con pip, relocatable)
#      y VERIFICA su SHA-256 contra el .sha256 publicado por astral-sh
#   2) instala requirements.lock DENTRO del portable con --require-hashes
#      (cada wheel verificada por hash = linaje completo del stack)
#   3) escribe la config de Jupyter SIN token (bind 127.0.0.1)
#   4) copia notebooks/ (si existe en la raiz del proyecto)
#   5) PRUEBA DE RELOCALIZACION: copia a otra ruta y verifica imports + GeoTIFF
#      real + pip funcional (los alumnos usaran %pip install en clase)
#   6) empaqueta el tarball + saca sha256
#
# El tarball NO incluye launcher: la arquitectura es UN solo SatLab.exe visual
# que descarga/extrae este tarball y lanza Jupyter (decision 2026-06-09).
#
# Uso:   powershell -ExecutionPolicy Bypass -File build_portable.ps1 [salida] [lockfile]
# =============================================================================
$ErrorActionPreference = "Stop"

# --- CONFIG ------------------------------------------------------------------
$PyVersion = "3.11.15"
$PbsTag    = "20260602"   # https://github.com/astral-sh/python-build-standalone/releases

# Auto-localizacion: funciona sin importar el CWD ni donde este la carpeta.
$Root = Split-Path $PSScriptRoot -Parent          # raiz de PortableSatelital
$Out  = if ($args.Count -ge 1) { $args[0] } else { Join-Path $Root "dist\portable" }
$Req  = if ($args.Count -ge 2) { $args[1] } else { (Join-Path $Root "requirements.lock") }

$triple = "x86_64-pc-windows-msvc"
$asset  = "cpython-$PyVersion+$PbsTag-$triple-install_only.tar.gz"
$url    = "https://github.com/astral-sh/python-build-standalone/releases/download/$PbsTag/$asset"

Write-Host "==> [1/6] Bajando $asset"
if (Test-Path $Out) { Remove-Item -Recurse -Force $Out }
New-Item -ItemType Directory -Force $Out | Out-Null
$tmp = Join-Path $env:TEMP $asset
if (-not (Test-Path $tmp)) {
  curl.exe -fsSL --retry 3 --retry-delay 2 -o $tmp $url
  if ($LASTEXITCODE -ne 0) { throw "descarga del interprete fallo (curl exit $LASTEXITCODE)" }
}

Write-Host "==> [1/6] Verificando SHA-256 del interprete contra astral-sh (SHA256SUMS)"
$sumsUrl = "https://github.com/astral-sh/python-build-standalone/releases/download/$PbsTag/SHA256SUMS"
$sums = (curl.exe -fsSL --retry 3 --retry-delay 2 $sumsUrl)
if ($LASTEXITCODE -ne 0) { throw "descarga de SHA256SUMS fallo (curl exit $LASTEXITCODE)" }
$line = $sums | Where-Object { $_ -match [regex]::Escape($asset) } | Select-Object -First 1
if (-not $line) { throw "el asset $asset no aparece en SHA256SUMS" }
$shaRemote = $line.Trim().Split(" ")[0].ToLower()
$shaLocal  = (Get-FileHash $tmp -Algorithm SHA256).Hash.ToLower()
if ($shaLocal -ne $shaRemote) {
  Remove-Item -Force $tmp
  throw "SHA-256 del interprete NO coincide (esperado $shaRemote, obtenido $shaLocal). Descarga corrupta o alterada."
}
Write-Host "    sha256 OK: $shaLocal"
tar -xzf $tmp -C $Out                      # extrae a  $Out\python\
$py = Join-Path $Out "python\python.exe"

Write-Host "==> [2/6] Instalando el stack con hashes verificados (sin venv)"
& $py -m pip install --upgrade pip
if ($Req -like "*.lock") {
  # Cada wheel se verifica contra su sha256 registrado: si PyPI entregara un
  # archivo distinto al congelado, la instalacion ABORTA.
  & $py -m pip install --require-hashes -r $Req
} else {
  Write-Host "    aviso: instalando sin --require-hashes (no es el .lock)"
  & $py -m pip install -r $Req
}
if ($LASTEXITCODE -ne 0) { throw "pip install fallo (exit $LASTEXITCODE)" }

Write-Host "==> [3/6] Config de Jupyter SIN token (bind 127.0.0.1)"
$cfgDir = Join-Path $Out "jupyter-config"
New-Item -ItemType Directory -Force $cfgDir | Out-Null
$jcfg = @"
# Jupyter SIN token (decision del proyecto). Seguro porque escucha SOLO en
# 127.0.0.1 (local; NO expuesto a la red de la oficina).
c.ServerApp.token = ''
c.ServerApp.password = ''
c.ServerApp.ip = '127.0.0.1'
c.ServerApp.open_browser = False
"@
$jcfg | Set-Content -Encoding utf8 (Join-Path $cfgDir "jupyter_server_config.py")

Write-Host "==> [4/6] Copiando notebooks/"
$nbSrc = Join-Path $Root "notebooks"
if (Test-Path $nbSrc) {
  Copy-Item -Recurse $nbSrc (Join-Path $Out "notebooks")
} else {
  New-Item -ItemType Directory -Force (Join-Path $Out "notebooks") | Out-Null
  Write-Host "    aviso: no hay notebooks/ en la raiz; el tarball lleva la carpeta vacia."
}

Write-Host "==> [5/6] PRUEBA DE RELOCALIZACION (lo que mas falla)"
$reloc = Join-Path $env:TEMP ("relocado_" + [System.Guid]::NewGuid().ToString("N"))
Copy-Item -Recurse $Out $reloc
$pyR = Join-Path $reloc "python\python.exe"
& $pyR -m jupyterlab --version
if ($LASTEXITCODE -ne 0) { throw "jupyterlab --version fallo tras mover" }
# pip DEBE funcionar movido (los alumnos usan %pip install en los notebooks)
& $pyR -m pip --version
if ($LASTEXITCODE -ne 0) { throw "pip no funciona tras mover" }
$testFile = Join-Path $env:TEMP "satlab_reloc_test.py"
@'
import numpy, rasterio, geopandas, numba, pyshepseg, exactextract, sklearn, scipy, pandas, matplotlib
assert numpy.__version__.startswith("1."), "numpy debe ser <2: " + numpy.__version__
# GeoTIFF REAL de ida y vuelta (no solo imports): GDAL + PROJ de las wheels
import os, tempfile
from rasterio.transform import from_origin
arr = (numpy.random.rand(2, 64, 64) * 255).astype("uint8")
path = os.path.join(tempfile.gettempdir(), "satlab_test.tif")
with rasterio.open(path, "w", driver="GTiff", height=64, width=64, count=2,
                   dtype="uint8", crs="EPSG:6372", transform=from_origin(2500000, 1200000, 30, 30)) as dst:
    dst.write(arr)
with rasterio.open(path) as src:
    back = src.read()
    assert back.shape == (2, 64, 64) and src.crs is not None
os.remove(path)
print("GDAL", rasterio.__gdal_version__, "| numpy", numpy.__version__, "| GeoTIFF OK | OK tras mover")
'@ | Set-Content -Encoding utf8 $testFile
& $pyR $testFile
if ($LASTEXITCODE -ne 0) { throw "prueba de relocalizacion fallo" }
Remove-Item -Force $testFile
Remove-Item -Recurse -Force $reloc

Write-Host "==> [6/6] Empaquetando"
$distDir = Join-Path $Root "dist"
New-Item -ItemType Directory -Force $distDir | Out-Null
$tarball = Join-Path $distDir "portable-satelital-windows-amd64.tar.gz"
if (Test-Path $tarball) { Remove-Item -Force $tarball }
# Empaca el CONTENIDO de $Out (python/, jupyter-config/, notebooks/) en la raiz
# del tar, para que SatLab.exe extraiga directo junto a si mismo.
tar -czf $tarball -C $Out python jupyter-config notebooks
if ($LASTEXITCODE -ne 0) { throw "tar fallo (exit $LASTEXITCODE)" }
$sha = (Get-FileHash $tarball -Algorithm SHA256).Hash.ToLower()
Write-Host ""
Write-Host "LISTO: $tarball"
Write-Host "sha256=$sha"
$sha | Set-Content -Encoding ascii "$tarball.sha256"
Write-Host "-> subelo a HF (LFS), registra la clave en manifest.txt (anti-clobber) y re-firma el manifest."
