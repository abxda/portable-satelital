#!/bin/sh
# Taller Offline (Linux / macOS).
# Canales OFICIALES, en orden de preferencia:
#   1. caddy  - instalado desde el repositorio de tu distro (Linux, GPG) o
#               Homebrew (macOS). Un solo comando, sin configuracion.
#   2. python3 - en Linux es parte del propio sistema operativo (no agrega
#               superficie nueva); en macOS, el oficial de python.org esta
#               firmado y notarizado por la Python Software Foundation.
# Sirve SOLO en 127.0.0.1, solo lectura, sin sudo. Ctrl+C para detener.
cd "$(dirname "$0")"
URL="http://127.0.0.1:8765/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb"
echo ""
echo "  Taller Offline sirviendo SOLO en este equipo:"
echo "  $URL"
echo "  (Ctrl+C para detener)"
echo ""
( sleep 2; xdg-open "$URL" 2>/dev/null || open "$URL" 2>/dev/null ) &

if command -v caddy >/dev/null 2>&1; then
  exec caddy file-server --root sitio --listen 127.0.0.1:8765
elif command -v python3 >/dev/null 2>&1; then
  exec python3 -m http.server 8765 --bind 127.0.0.1 -d sitio
else
  echo "Falta un servidor. Pide a TI instalar 'caddy' desde el repositorio"
  echo "oficial de la distro (apt/dnf) o Homebrew en macOS."
  exit 1
fi
