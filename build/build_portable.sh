#!/usr/bin/env bash
# =============================================================================
# build_portable.sh — construye el Python portable (Linux / macOS) de
# PortableSatelital. CORRE EN EL SO/ARCH DESTINO (no cross-buildea: las wheels
# de GDAL/numba son binarias por plataforma).
#
# Qué hace:
#   1) baja python-build-standalone (CPython portable, con pip, relocatable)
#   2) instala el requirements.txt DENTRO del portable (sin activar venv)
#   3) escribe la config de Jupyter SIN token (bind 127.0.0.1)
#   4) PRUEBA DE RELOCALIZACIÓN: copia a otra ruta y verifica que importa y lee GDAL
#   5) empaqueta el tarball + saca sha256
#
# Uso:   ./build_portable.sh [carpeta_salida] [requirements.txt]
# Ej:    ./build_portable.sh portable ../requirements.txt
# =============================================================================
set -euo pipefail

# --- CONFIG (ajusta) ---------------------------------------------------------
PY_VERSION="3.11.9"     # versión de CPython
PBS_TAG="20240415"      # release de python-build-standalone — VERIFICA EL ÚLTIMO:
                        # https://github.com/astral-sh/python-build-standalone/releases
# Auto-localización: funciona sin importar desde dónde se invoque ni dónde esté
# la carpeta PortableSatelital (muévela a D:\, USB, otra máquina — da igual).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(dirname "$SCRIPT_DIR")"        # raíz de PortableSatelital (padre de build/)
OUT="${1:-portable}"                   # salida (relativa a tu CWD)
REQ="${2:-$ROOT/requirements.txt}"     # requirements.txt auto-localizado en la carpeta

# --- detectar SO / arch / triple --------------------------------------------
s="$(uname -s)"; m="$(uname -m)"
case "$s" in
  Linux)  os="linux"
          case "$m" in
            x86_64)        arch="amd64"; triple="x86_64-unknown-linux-gnu";;
            aarch64|arm64) arch="arm64"; triple="aarch64-unknown-linux-gnu";;
            *) echo "arch no soportada: $m"; exit 1;; esac ;;
  Darwin) os="macos"
          case "$m" in
            arm64)  arch="arm64"; triple="aarch64-apple-darwin";;
            x86_64) arch="amd64"; triple="x86_64-apple-darwin";;
            *) echo "arch no soportada: $m"; exit 1;; esac ;;
  *) echo "Este script es para Linux/macOS. En Windows usa build_portable.ps1"; exit 1;;
esac
asset="cpython-${PY_VERSION}+${PBS_TAG}-${triple}-install_only.tar.gz"
url="https://github.com/astral-sh/python-build-standalone/releases/download/${PBS_TAG}/${asset}"

echo "==> SO=$os arch=$arch triple=$triple"
echo "==> [1/5] Bajando $asset"
rm -rf "$OUT"; mkdir -p "$OUT"
curl -fL -o "/tmp/$asset" "$url"
tar -xzf "/tmp/$asset" -C "$OUT"          # extrae a  $OUT/python/
PY="$OUT/python/bin/python3"

echo "==> [2/5] Instalando el stack (sin activar nada)"
"$PY" -m pip install --upgrade pip
"$PY" -m pip install -r "$REQ"            # requirements.txt trae numpy<2; GDAL viene en las wheels
chmod -R +x "$OUT/python/bin"

echo "==> [3/5] Config de Jupyter SIN token (bind 127.0.0.1)"
mkdir -p "$OUT/jupyter-config"
cat > "$OUT/jupyter-config/jupyter_server_config.py" <<'CFG'
# Jupyter SIN token (decisión del proyecto). Seguro porque escucha SOLO en 127.0.0.1.
c.ServerApp.token = ''
c.ServerApp.password = ''
c.ServerApp.ip = '127.0.0.1'
c.ServerApp.open_browser = False
CFG

echo "==> [4/5] PRUEBA DE RELOCALIZACIÓN (lo que más falla)"
TMP="$(mktemp -d)/relocado"
cp -a "$OUT" "$TMP"
"$TMP/python/bin/python3" -m jupyterlab --version
"$TMP/python/bin/python3" - <<'PYTEST'
import numpy, rasterio, geopandas, numba, pyshepseg, exactextract, sklearn
assert numpy.__version__.startswith("1."), "numpy debe ser <2, es: " + numpy.__version__
print("GDAL", rasterio.__gdal_version__, "| numpy", numpy.__version__, "| OK tras mover")
PYTEST
rm -rf "$TMP"

echo "==> [5/5] Empaquetando"
# OJO: antes del tarball, agrega a  $OUT/  el binario  sat-launcher  (compilado aparte)
# y la carpeta  notebooks/  (tutorial + datos de ejemplo).
if [ ! -e "$OUT/sat-launcher" ]; then
  echo "    ⚠ aviso: no encuentro $OUT/sat-launcher — empaco solo el python (agrega el launcher y re-empaca)."
fi
if [ "$os" = "macos" ]; then
  xattr -dr com.apple.quarantine "$OUT" 2>/dev/null || true   # Gatekeeper
  echo "    (macOS: recuerda firmar ad-hoc el binario sat-launcher cuando lo agregues)"
fi
tarball="portable-satelital-${os}-${arch}.tar.gz"
tar -czf "$tarball" "$OUT"
if command -v sha256sum >/dev/null 2>&1; then sha=$(sha256sum "$tarball" | cut -d' ' -f1)
else sha=$(shasum -a 256 "$tarball" | cut -d' ' -f1); fi
echo ""
echo "LISTO: $tarball"
echo "sha256=$sha"
echo "-> súbelo a HF (LFS) y registra la clave en manifest.txt (anti-clobber)."
