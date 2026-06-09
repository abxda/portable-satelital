# MISIÓN: construir el Laboratorio Satelital Portable para Linux x64

**Para:** el agente que corre en la estación Ubuntu del Dr. Abel Coronado.
**Tu rol:** eres un **agente constructor remoto**. Construyes y pruebas; **NO publicas
ni firmas nada**. El líder (en la máquina Windows) tiene la llave de firma del catálogo
y es el único que toca `manifest.txt`.
**Entregable final:** una carpeta `entregables/` + el archivo `REPORTE_LINUX.md` lleno
(la plantilla viene junto a este documento). El Dr. se lleva eso de regreso.

Lee TODO este documento antes de ejecutar nada. Sigue las fases EN ORDEN. Cada fase
tiene un **criterio de éxito** explícito: no avances si no se cumple, y si algo falla,
anota el error completo en el reporte y sigue las instrucciones de "si falla".

---

## Contexto en 5 líneas

Es un laboratorio educativo: **JupyterLab + stack geoespacial de Python** (rasterio,
geopandas, pyshepseg, numba, scikit-learn…) empacado como **carpeta portable**
(python-build-standalone, sin conda, sin instalador). En Windows ya existe y funciona:
un launcher visual (`SatLab`) descarga el paquete desde Hugging Face (catálogo firmado
Ed25519 + SHA-256) y abre Jupyter en `127.0.0.1`. **Falta la versión Linux x64**, que es
exactamente lo que tú vas a construir. Todo el código y las recetas ya están en GitHub.

## Reglas duras (no negociables)

1. **NUNCA** imprimas, hagas `echo`, ni commitees tokens o llaves. Si tienes credenciales
   en tu entorno, úsalas solo si una fase lo pide explícitamente.
2. **NO** edites ni subas `manifest.txt` ni nada al dataset de Hugging Face ni al repo de
   GitHub. Eso lo hace el líder (necesita firmar con una llave que tú no tienes).
3. Trabaja en una ruta **SIN espacios** (p. ej. `~/satlab-build`).
4. No marques una fase como exitosa solo porque el comando salió con código 0:
   verifica el **resultado** pedido en el criterio de éxito.
5. Necesitas ~**6 GB libres** de disco y conexión a internet.

---

## FASE 0 — Diagnóstico del entorno

```bash
uname -m            # criterio: x86_64
lsb_release -ds     # anota la versión de Ubuntu en el reporte
df -h ~ | tail -1   # criterio: >= 6 GB libres
which git curl tar  # criterio: los tres existen
```

✅ **Éxito:** x86_64 + 6 GB + git/curl/tar. Anota todo en el reporte.
❌ **Si falla:** detente y repórtalo; no tiene caso continuar.

## FASE 1 — Clonar el repositorio (es público, no necesita token)

```bash
mkdir -p ~/satlab-build && cd ~/satlab-build
git clone https://github.com/abxda/portable-satelital.git
cd portable-satelital
git log --oneline -1   # anota el commit en el reporte
```

Lecturas obligatorias antes de seguir (están en el repo, son cortas):
- `build/README.md` — la receta y tu rol como agente remoto.
- `build/build_portable.sh` — el script que vas a correr (léelo completo).

## FASE 2 — Construir el Python portable de Linux

```bash
cd ~/satlab-build/portable-satelital
chmod +x build/build_portable.sh
./build/build_portable.sh
```

Esto tarda **10–25 minutos** (descarga CPython 3.11.15 verificado contra el SHA256SUMS
oficial de astral-sh + instala ~117 paquetes). Termina con:

```
LISTO: portable-satelital-linux-amd64.tar.gz
sha256=<64 hex>
```

✅ **Éxito:** existe `portable-satelital-linux-amd64.tar.gz` + su `.sha256`, y la
"PRUEBA DE RELOCALIZACIÓN" imprimió `GDAL ... | GeoTIFF OK | OK tras mover`.
**Copia esa salida completa al reporte.**

❌ **Si falla en el paso [2/6] (pip) con errores de hashes o de `pywinpty`:** es
esperado — `requirements.lock` se generó en Windows e incluye un paquete solo-Windows.
Regenera el lock PARA LINUX usando el propio Python 3.11 del portable (NO uses el
python del sistema, puede ser 3.12 y resolvería mal):

```bash
./portable/python/bin/python3 -m pip install pip-tools
./portable/python/bin/python3 -m piptools compile --generate-hashes --strip-extras \
    -o requirements-linux.lock requirements.txt
./build/build_portable.sh portable requirements-linux.lock
```

Si lo hiciste, **incluye `requirements-linux.lock` en los entregables** (el líder lo
versiona en el repo para auditoría). Cualquier otro error: pégalo completo en el
reporte y detén la fase.

## FASE 3 — Probar como lo viviría un alumno (en OTRA ruta)

```bash
mkdir -p /tmp/satlab-prueba && cd /tmp/satlab-prueba
tar -xzf ~/satlab-build/portable-satelital/portable-satelital-linux-amd64.tar.gz
./python/bin/python3 -m jupyterlab --version     # criterio: imprime 4.x
./python/bin/python3 -m pip --version            # criterio: pip funciona (los alumnos usan %pip)
```

Ahora la prueba estrella — **ejecutar el cuaderno de bienvenida completo** (esto genera
el cuaderno-evidencia que el Dr. se lleva a Windows):

```bash
cd /tmp/satlab-prueba/notebooks
../python/bin/python3 -m jupyter nbconvert --to notebook --execute 00_Bienvenida.ipynb \
    --output 00_Bienvenida_ejecutado_linux.ipynb
```

✅ **Éxito:** se crea `00_Bienvenida_ejecutado_linux.ipynb` SIN errores (nbconvert se
queja en stderr si una celda truena). Ese cuaderno ejecutado va a `entregables/`.

Prueba opcional pero recomendada (Jupyter de verdad, 10 segundos):

```bash
cd /tmp/satlab-prueba
JUPYTER_CONFIG_DIR=$PWD/jupyter-config ./python/bin/python3 -m jupyterlab \
    --no-browser --port 8899 &
sleep 8
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8899/lab   # criterio: 200
kill %1
```

## FASE 4 (opcional, no bloquea) — Compilar el launcher SatLab para Linux

El código ya compila en Linux (`satlab-launcher/`), pero la GUI necesita toolchain:

```bash
# Dependencias del sistema (Ubuntu 24.04 usa webkit2gtk-4.1):
sudo apt-get update && sudo apt-get install -y build-essential pkg-config \
    libgtk-3-dev libwebkit2gtk-4.1-dev

# Go >= 1.24 (si no lo tienes):  https://go.dev/dl/  (o usa el del sistema si alcanza)
go version

# Wails CLI:
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
export PATH="$PATH:$(go env GOPATH)/bin"

cd ~/satlab-build/portable-satelital/satlab-launcher
# Ubuntu 24.04 (webkit 4.1):
wails build -platform linux/amd64 -tags webkit2_41
# (Ubuntu 22.04 con libwebkit2gtk-4.0-dev: quita el -tags)
# -> build/bin/SatLab
```

Prueba sin GUI (usa el laboratorio ya extraído de la FASE 3):

```bash
cp build/bin/SatLab /tmp/satlab-prueba/
cd /tmp/satlab-prueba && ./SatLab --headless-smoke; echo "exit=$?"
cat satlab-headless.log    # criterio: "SMOKE OK" y exit=0
```

Si tu sesión tiene escritorio, lanza también `./SatLab` (GUI), espera 5 s y verifica que
`satlab.log` contenga **"UI LISTA"**; ciérralo después.

✅ **Éxito:** binario `SatLab` + `SMOKE OK`. Súmalo a entregables con su sha256.
❌ **Si falla:** anota el error y sigue; el portable (FASES 2–3) es lo importante.

## FASE 5 — Armar entregables y reporte

```bash
mkdir -p ~/satlab-build/entregables && cd ~/satlab-build/entregables
cp ~/satlab-build/portable-satelital/portable-satelital-linux-amd64.tar.gz .
cp ~/satlab-build/portable-satelital/portable-satelital-linux-amd64.tar.gz.sha256 .
cp /tmp/satlab-prueba/notebooks/00_Bienvenida_ejecutado_linux.ipynb .
# si existen:
cp ~/satlab-build/portable-satelital/requirements-linux.lock . 2>/dev/null || true
cp ~/satlab-build/portable-satelital/satlab-launcher/build/bin/SatLab . 2>/dev/null || true
sha256sum * > SHA256SUMS_entregables.txt
```

Llena **`REPORTE_LINUX.md`** (la plantilla está junto a esta misión) y cópialo también a
`entregables/`. El criterio global de éxito de TODA la misión es: el reporte permite al
líder registrar la clave `linux-amd64-portable` en el catálogo **sin hacerte ninguna
pregunta**.

> **Subida opcional:** SOLO si el Dr. te dice explícitamente que subas el tarball a
> Hugging Face con tus credenciales, hazlo con
> `hf upload abxda/portable-satelital <tarball> portable-satelital-linux-amd64.tar.gz --repo-type dataset`
> y anota en el reporte el `oid` LFS (debe ser igual al sha256). El manifest lo firma y
> sube el líder SIEMPRE.
