#!/bin/sh
# Taller Offline (Linux / macOS): usa el python3 del sistema.
#   Linux: python3 viene en practicamente toda distribucion.
#   macOS: si falta, instala el oficial de python.org (firmado/notarizado
#          por la Python Software Foundation).
# Sirve SOLO en 127.0.0.1, solo lectura, sin sudo. Ctrl+C para detener.
cd "$(dirname "$0")"
URL="http://127.0.0.1:8765/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb"
echo ""
echo "  Taller Offline sirviendo SOLO en este equipo:"
echo "  $URL"
echo "  (Ctrl+C para detener)"
echo ""
( sleep 2; xdg-open "$URL" 2>/dev/null || open "$URL" 2>/dev/null ) &
exec python3 -m http.server 8765 --bind 127.0.0.1 -d sitio
