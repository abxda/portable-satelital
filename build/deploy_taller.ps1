# Construye el sitio del taller (JupyterLite) + portada/teoria y lo publica en
# gh-pages. USO:  powershell -File build\deploy_taller.ps1 [-NoPush]
#
# Pasos: (1) staging de contenidos, (2) jupyter lite build con overrides,
# (3) overlay de portada/teoria INYECTANDO el bloque jupyter-config-data del
# index.html generado (JupyterLite hereda config de los index padres; una
# portada sin ese bloque rompe /lab), (4) push -f a gh-pages.
param([switch]$NoPush)
$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
$Wasm = Join-Path $Root "wasm"
$Site = Join-Path $Root "dist\taller-site"
$Jupyter = Join-Path $Root "dist\qaenv\Scripts\jupyter.exe"

Write-Host "==> [1/4] staging de contenidos"
$C = Join-Path $Wasm "_contents"
if (Test-Path $C) { Remove-Item -Recurse -Force $C }
New-Item -ItemType Directory -Force $C | Out-Null
Copy-Item (Join-Path $Wasm "taller\Taller_ML_Urbano_WASM.ipynb") $C
Copy-Item (Join-Path $Wasm "taller\Mi_Lienzo.ipynb") $C
# shepherd va como WHEEL (piplite): %pip install -q shepherd-wasm.
# Los DATOS estatales NO van en el sitio (GitHub Pages topa en 100MB/archivo):
# el cuaderno los trae de mis_datos (kit) o de Hugging Face (en linea).

Write-Host "==> [2/4] jupyter lite build"
if (Test-Path $Site) { Remove-Item -Recurse -Force $Site }
# limpiar el cache de doit: si no, omite tareas (p.ej. extraer pyodide) aunque
# el output-dir se haya borrado
Remove-Item (Join-Path $Wasm ".jupyterlite.doit.db") -Force -ErrorAction SilentlyContinue
$Wheel = (Get-ChildItem (Join-Path $Root "dist\wheels\shepherd_wasm-*.whl") | Select-Object -First 1).FullName
Push-Location $Wasm
& $Jupyter lite build --contents _contents --settings-overrides overrides.json --piplite-wheels $Wheel --output-dir $Site
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "jupyter lite build fallo" }
Pop-Location

Write-Host "==> [3/4] overlay portada/teoria con jupyter-config-data inyectado"
# OJO: -Encoding UTF8 SIEMPRE. PowerShell 5.1 sin -Encoding lee como ANSI y
# convierte la portada UTF-8 en mojibake (bug real del primer deploy).
$built = Get-Content (Join-Path $Site "index.html") -Raw -Encoding UTF8
$m = [regex]::Match($built, '(?s)<script id="jupyter-config-data"[^>]*>.*?</script>')
if (-not $m.Success) { throw "no encontre jupyter-config-data en el index generado" }
$portada = Get-Content (Join-Path $Wasm "web\index.html") -Raw -Encoding UTF8
if ($portada -notmatch "<!--JUPYTER_CONFIG-->") { throw "portada sin placeholder JUPYTER_CONFIG" }
# .Replace (no -replace): el JSON trae caracteres que -replace interpreta
$final = $portada.Replace("<!--JUPYTER_CONFIG-->", $m.Value)
[IO.File]::WriteAllText((Join-Path $Site "index.html"), $final,
                        (New-Object System.Text.UTF8Encoding $false))
# sanity de encoding, construido en ASCII puro (este .ps1 sin BOM se lee como
# ANSI en PS 5.1: un literal acentuado aqui se corrompe y el match miente):
$check = Get-Content (Join-Path $Site "index.html") -Raw -Encoding UTF8
$acentuada = "An" + [char]0xE1 + "lisis"      # 'Analisis' con a acentuada
$mojibake = [string][char]0xC3                 # firma tipica de UTF-8 leido como ANSI
if (-not $check.Contains($acentuada)) { throw "encoding roto: falta texto acentuado" }
if ($check.Contains($mojibake + [char]0x83)) { throw "encoding roto: mojibake detectado" }
Copy-Item (Join-Path $Wasm "web\teoria.html") $Site

# sanity: el index final DEBE tener el bloque
if ((Get-Content (Join-Path $Site "index.html") -Raw) -notmatch "jupyter-config-data") {
  throw "el index final quedo sin jupyter-config-data"
}
Write-Host "    config heredable OK"

if ($NoPush) { Write-Host "==> listo (sin push)"; exit 0 }

Write-Host "==> [4/4] push a gh-pages"
Push-Location $Site
git init -b gh-pages 2>&1 | Out-Null
git add -A 2>$null
git commit -q -m "deploy taller + portada + teoria"
git remote add origin git@github.com:abxda/portable-satelital.git 2>$null
$env:GIT_SSH_COMMAND = 'ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15'
git push -f origin gh-pages
Pop-Location
Write-Host "LISTO: https://abxda.github.io/portable-satelital/"
