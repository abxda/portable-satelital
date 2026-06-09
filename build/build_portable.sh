#!/usr/bin/env bash
# =============================================================================
# build_portable.sh — construye el Python portable (Linux / macOS) de
# PortableSatelital. CORRE EN EL SO/ARCH DESTINO (no cross-buildea: las wheels
# de GDAL/numba son binarias por plataforma).
#
# Qué hace (espejo de build_portable.ps1, la receta canónica):
#   1) baja python-build-standalone y VERIFICA su SHA-256 contra el SHA256SUMS
#      oficial del release de astral-sh
#   2) instala requirements.lock DENTRO del portable con --require-hashes
#      (cada wheel verificada por hash = linaje completo del stack)
#      ⚠ OJO: requirements.lock se generó en Windows; si pip falla por hashes
#      faltantes de TU plataforma, re-genera el lock aquí:
#         "$PY" -m pip install pip-tools && \
#         "$PY" -m piptools compile --generate-hashes --strip-extras \
#               -o requirements-$(uname -s).lock requirements.txt
#      y usa ese lock (repórtalo al líder para auditoría).
#   3) escribe la config de Jupyter SIN token (bind 127.0.0.1)
#   4) copia notebooks/ (si existe en la raíz del proyecto)
#   5) PRUEBA DE RELOCALIZACIÓN: copia a otra ruta y verifica imports + GeoTIFF
#      real + pip funcional (los alumnos usan %pip install en clase)
#   6) empaqueta el tarball (preservando +x y symlinks) + saca sha256
#
# El tarball NO incluye launcher: la arquitectura es UN solo launcher visual
# (SatLab) que descarga/extrae este tarball y lanza Jupyter.
#
# Uso:   ./build_portable.sh [carpeta_salida] [lockfile]
# =============================================================================
set -euo pipefail

# --- CONFIG ------------------------------------------------------------------
PY_VERSION="3.11.15"
PBS_TAG="20260602"      # https://github.com/astral-sh/python-build-standalone/releases

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(dirname "$SCRIPT_DIR")"
OUT="${1:-portable}"
REQ="${2:-$ROOT/requirements.lock}"

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
echo "==> [1/6] Bajando $asset"
rm -rf "$OUT"; mkdir -p "$OUT"
curl -fL --retry 3 --retry-delay 2 -o "/tmp/$asset" "$url"

echo "==> [1/6] Verificando SHA-256 del intérprete contra astral-sh (SHA256SUMS)"
curl -fsSL --retry 3 --retry-delay 2 \
  -o /tmp/PBS_SHA256SUMS \
  "https://github.com/astral-sh/python-build-standalone/releases/download/${PBS_TAG}/SHA256SUMS"
want="$(grep "  ${asset}\$" /tmp/PBS_SHA256SUMS | cut -d' ' -f1 || true)"
[ -n "$want" ] || { echo "el asset no aparece en SHA256SUMS"; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "/tmp/$asset" | cut -d' ' -f1)
else got=$(shasum -a 256 "/tmp/$asset" | cut -d' ' -f1); fi
[ "$want" = "$got" ] || { echo "SHA-256 NO coincide (esperado $want, obtenido $got)"; exit 1; }
echo "    sha256 OK: $got"
tar -xzf "/tmp/$asset" -C "$OUT"          # extrae a  $OUT/python/
PY="$OUT/python/bin/python3"

echo "==> [2/6] Instalando el stack con hashes verificados (sin venv)"
"$PY" -m pip install --upgrade pip
case "$REQ" in
  *.lock) "$PY" -m pip install --require-hashes -r "$REQ" ;;
  *)      echo "    aviso: instalando SIN --require-hashes (no es el .lock)"
          "$PY" -m pip install -r "$REQ" ;;
esac
chmod -R +x "$OUT/python/bin"

echo "==> [3/6] Config de Jupyter SIN token (bind 127.0.0.1)"
mkdir -p "$OUT/jupyter-config"
cat > "$OUT/jupyter-config/jupyter_server_config.py" <<'CFG'
# Jupyter SIN token (decisión del proyecto). Seguro porque escucha SOLO en
# 127.0.0.1 (local; NO expuesto a la red de la oficina).
c.ServerApp.token = ''
c.ServerApp.password = ''
c.ServerApp.ip = '127.0.0.1'
c.ServerApp.open_browser = False
CFG

echo "==> [4/6] Copiando notebooks/"
if [ -d "$ROOT/notebooks" ]; then
  cp -a "$ROOT/notebooks" "$OUT/notebooks"
else
  mkdir -p "$OUT/notebooks"
  echo "    aviso: no hay notebooks/ en la raíz; el tarball lleva la carpeta vacía."
fi

echo "==> [5/6] PRUEBA DE RELOCALIZACIÓN (lo que más falla)"
TMP="$(mktemp -d)/relocado"
cp -a "$OUT" "$TMP"
"$TMP/python/bin/python3" -m jupyterlab --version
"$TMP/python/bin/python3" -m pip --version    # %pip de los alumnos debe funcionar movido
"$TMP/python/bin/python3" - <<'PYTEST'
import numpy, rasterio, geopandas, numba, pyshepseg, exactextract, sklearn, scipy, pandas, matplotlib
assert numpy.__version__.startswith("1."), "numpy debe ser <2, es: " + numpy.__version__
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
PYTEST
rm -rf "$TMP"

echo "==> [6/6] Empaquetando"
if [ "$os" = "macos" ]; then
  xattr -dr com.apple.quarantine "$OUT" 2>/dev/null || true   # Gatekeeper
fi
tarball="portable-satelital-${os}-${arch}.tar.gz"
# Empaca el CONTENIDO (python/, jupyter-config/, notebooks/) en la raíz del tar,
# preservando symlinks y bits +x (imprescindible en macOS/Linux).
tar -czf "$tarball" -C "$OUT" python jupyter-config notebooks
if command -v sha256sum >/dev/null 2>&1; then sha=$(sha256sum "$tarball" | cut -d' ' -f1)
else sha=$(shasum -a 256 "$tarball" | cut -d' ' -f1); fi
echo "$sha" > "$tarball.sha256"
echo ""
echo "LISTO: $tarball"
echo "sha256=$sha"
echo "-> repórtalo al líder: él lo sube a HF, registra la clave en manifest.txt y re-firma."
