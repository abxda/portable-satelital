#!/usr/bin/env bash
# QA del binario Linux PUBLICADO, dentro de un contenedor Ubuntu 24.04 limpio
# (vía podman desde Windows). Prueba el flujo real del alumno en headless:
#   1. instalar deps de runtime de la GUI (webkit/gtk: el binario las linkea)
#      + un PROJ VIEJO del sistema (proj-bin/proj-data) para el escenario hostil
#   2. descargar SatLab-linux-amd64 del release
#   3. --headless-install   (catalogo firmado + sha256 + extraccion, desde HF)
#   4. --headless-envcheck  con entorno limpio
#   5. --headless-envcheck  con PROJ_LIB/PROJ_DATA/GDAL_DATA ENVENENADAS al
#      /usr/share/proj REAL del sistema (la regresion del bug del Dr.)
#   6. --headless-smoke     (Jupyter HTTP 200)
set -euo pipefail
TAG="${1:?uso: qa_linux_container.sh vX.Y.Z}"

export DEBIAN_FRONTEND=noninteractive
echo "==> deps (webkit/gtk runtime + proj viejo del sistema para el escenario hostil)"
apt-get update -qq
apt-get install -y -qq curl ca-certificates libwebkit2gtk-4.1-0 libgtk-3-0 proj-bin proj-data > /dev/null
ls /usr/share/proj/proj.db && echo "    proj.db VIEJO del sistema presente (perfecto para la prueba)"

# Como un ALUMNO de verdad: usuario sin privilegios (ademas Jupyter se niega a
# correr como root, que es lo unico que hay en un contenedor pelon).
useradd -m alumno
mkdir -p /satlab && chown alumno:alumno /satlab
cd /satlab
echo "==> bajando SatLab-linux-amd64 $TAG"
runuser -u alumno -- curl -fsSL -o SatLab "https://github.com/abxda/portable-satelital/releases/download/$TAG/SatLab-linux-amd64"
chmod +x SatLab

echo "==> [1/4] headless-install (descarga real desde HF, catalogo firmado)"
runuser -u alumno -- ./SatLab --headless-install
grep -q "INSTALL OK" satlab-headless.log && echo "    INSTALL OK"

echo "==> [2/4] envcheck con entorno limpio"
runuser -u alumno -- ./SatLab --headless-envcheck
grep -q "ENVCHECK OK" satlab-headless.log && echo "    ENVCHECK limpio OK"

echo "==> [3/4] envcheck con entorno ENVENENADO (proj viejo real del sistema)"
runuser -u alumno -- env PROJ_LIB=/usr/share/proj PROJ_DATA=/usr/share/proj GDAL_DATA=/usr/share/gdal \
  ./SatLab --headless-envcheck
grep -q "ENVCHECK OK" satlab-headless.log && echo "    ENVCHECK envenenado OK"

echo "==> [4/4] smoke (Jupyter HTTP 200)"
runuser -u alumno -- ./SatLab --headless-smoke
grep -q "SMOKE OK" satlab-headless.log && echo "    SMOKE OK"

echo ""
echo "===== bitacora completa ====="
cat satlab-headless.log
echo "===== QA LINUX EN CONTENEDOR: TODO OK ====="
