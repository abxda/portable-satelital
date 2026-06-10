# =============================================================================
# servir_taller.ps1 - mini-servidor del Taller Offline.
#
# PARA EL AREA DE CIBERSEGURIDAD (lectura de 2 minutos):
#   * Lo unico que se EJECUTA es powershell.exe (firmado por Microsoft) y la
#     clase TcpListener de .NET (parte de Windows). No hay binarios de terceros.
#   * Escucha SOLO en 127.0.0.1 (loopback): nada entra ni sale de la red.
#   * Solo responde GET/HEAD de archivos DENTRO de la carpeta del taller
#     (bloquea rutas con '..'). No escribe, no ejecuta, no sube nada.
#   * Sin privilegios de administrador, sin reglas de firewall, sin registro.
#   * Para detenerlo: cerrar la ventana o Ctrl+C.
# (ASCII puro a proposito: PS 5.1 lee .ps1 sin BOM como ANSI.)
# =============================================================================
param([int]$Puerto = 8765, [string]$Carpeta = "")

$ErrorActionPreference = "Stop"
if ($Carpeta -eq "") { $Carpeta = Join-Path $PSScriptRoot "sitio" }
$Carpeta = (Resolve-Path $Carpeta).Path
if (-not (Test-Path (Join-Path $Carpeta "lab\index.html"))) {
  Write-Host "No encuentro el sitio del taller en: $Carpeta"; exit 1
}

$tipos = @{
  ".html"="text/html; charset=utf-8"; ".js"="text/javascript"; ".mjs"="text/javascript";
  ".css"="text/css"; ".json"="application/json"; ".ipynb"="application/json";
  ".wasm"="application/wasm"; ".png"="image/png"; ".svg"="image/svg+xml";
  ".ico"="image/x-icon"; ".woff2"="font/woff2"; ".woff"="font/woff";
  ".map"="application/json"; ".whl"="application/octet-stream";
  ".zip"="application/zip"; ".tif"="application/octet-stream";
  ".bz2"="application/octet-stream"; ".data"="application/octet-stream";
  ".txt"="text/plain"; ".md"="text/plain"
}

$listener = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, $Puerto)
$listener.Start()
$url = "http://127.0.0.1:$Puerto/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb"
Write-Host ""
Write-Host "  Taller Offline sirviendo SOLO en este equipo:"
Write-Host "  $url"
Write-Host "  (cierra esta ventana para detenerlo)"
Write-Host ""
Start-Process $url | Out-Null

$enc = [System.Text.Encoding]::ASCII
while ($true) {
  $cliente = $listener.AcceptTcpClient()
  try {
    $st = $cliente.GetStream()
    $st.ReadTimeout = 5000
    # lee la linea de peticion (hasta \n)
    $buf = New-Object byte[] 4096
    $n = $st.Read($buf, 0, $buf.Length)
    if ($n -le 0) { $cliente.Close(); continue }
    $linea = ($enc.GetString($buf, 0, $n) -split "`r`n")[0]
    $partes = $linea -split " "
    $metodo = $partes[0]; $ruta = $partes[1]
    if ($ruta -eq $null) { $cliente.Close(); continue }
    $ruta = [System.Uri]::UnescapeDataString(($ruta -split "\?")[0])
    if ($ruta.EndsWith("/")) { $ruta = $ruta + "index.html" }

    $ok = ($metodo -eq "GET" -or $metodo -eq "HEAD") -and ($ruta -notmatch "\.\.")
    $archivo = Join-Path $Carpeta ($ruta.TrimStart("/") -replace "/", "\")
    if ($ok -and (Test-Path $archivo -PathType Leaf)) {
      $bytes = [System.IO.File]::ReadAllBytes($archivo)
      $ext = [System.IO.Path]::GetExtension($archivo).ToLower()
      $tipo = $tipos[$ext]; if (-not $tipo) { $tipo = "application/octet-stream" }
      $hdr = "HTTP/1.1 200 OK`r`nContent-Type: $tipo`r`nContent-Length: $($bytes.Length)`r`nCache-Control: no-cache`r`nConnection: close`r`n`r`n"
      $hb = $enc.GetBytes($hdr)
      $st.Write($hb, 0, $hb.Length)
      if ($metodo -eq "GET") { $st.Write($bytes, 0, $bytes.Length) }
    } else {
      $cuerpo = $enc.GetBytes("404")
      $hdr = "HTTP/1.1 404 Not Found`r`nContent-Type: text/plain`r`nContent-Length: $($cuerpo.Length)`r`nConnection: close`r`n`r`n"
      $hb = $enc.GetBytes($hdr)
      $st.Write($hb, 0, $hb.Length); $st.Write($cuerpo, 0, $cuerpo.Length)
    }
    $st.Flush()
  } catch { }
  finally { $cliente.Close() }
}
