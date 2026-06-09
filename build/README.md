# build/ — construir los Python portables + modelo de agentes remotos

## Qué hay aquí
- **`build_portable.sh`** — Linux / macOS (corre en el SO destino)
- **`build_portable.ps1`** — Windows x64

Cada script: baja **python-build-standalone**, instala el `../requirements.txt` **dentro** del portable
(sin venv), escribe la config de **Jupyter sin token** (bind 127.0.0.1), corre la **prueba de
relocalización**, y empaqueta el tarball + `sha256`.

```bash
# Linux / macOS  (corre EN esa máquina)
chmod +x build/build_portable.sh
./build/build_portable.sh portable ./requirements.txt

# Windows  (corre EN Windows)
powershell -ExecutionPolicy Bypass -File build\build_portable.ps1 portable .\requirements.txt
```

> Antes del tarball final, agrega a `portable/`: el binario **`sat-launcher`** (compilado aparte, ver
> README §5) y la carpeta **`notebooks/`** (tutorial + datos). Luego re-empaca.

---

## ⚠️ Regla dura: el PYTHON portable se construye EN cada SO/arch (no se cross-buildea)
`pip` baja **wheels binarias específicas de la plataforma** (el GDAL de rasterio, el LLVM de numba). **NO**
puedes hornear el **python portable** de Mac desde Windows ni al revés. **Cada tarball se construye en su plataforma.**

> *Esto aplica SOLO al python portable.* Los **binarios Go** (meta-launcher y sat-launcher) son **Go puro
> (CGO=0)** y **SÍ se cross-compilan desde una sola máquina** con `GOOS=… GOARCH=… go build`. En esta
> máquina (Windows, Go 1.26.2) generas los binarios Go de los 3 SO. Ver README principal, sección
> «Entorno YA disponible en esta máquina».

---

## 🧭 TÚ eres el LÍDER de agentes remotos

Este es el modelo de trabajo (el mismo que usó el Big Data lab y que funcionó):

- **Tú (el agente que recibe PortableSatelital) eres el LÍDER / orquestador.** Probablemente solo tengas
  acceso a UN sistema operativo. **No construyes los 3 tú mismo.**
- **Despachas una COPIA de estas instrucciones** (al menos `build/` + `requirements.txt` + el README) a un
  **agente remoto** en cada SO objetivo (Windows, Linux, macOS Apple Silicon).
- **Cada agente remoto** corre el `build_portable.*` de su SO → obtiene su `portable-satelital-<os>-<arch>.tar.gz`
  + su `sha256` → y **te reporta el sha** (o publica él mismo el tarball a HF y te reporta el `oid` LFS).
- **TÚ (líder) eres dueño del `manifest.txt`:** registras el sha de cada plataforma (**aditivo,
  anti-clobber**), publicas los launchers + descriptores, y **verificas end-to-end** en cada SO. *(Disciplina
  del Big Data: los agentes publican artefactos + reportan sha; el LÍDER edita el manifest.)*

### Plantilla de prompt para un agente remoto
```
PARA: Agente <SO> (Windows / Linux / macOS arm64). [con git+HF si va a publicar él]
OBJETIVO: construir el Python portable de PortableSatelital para <SO>/<arch> y reportar el sha256.
PASOS:
  1. Recibe la carpeta PortableSatelital (al menos build/ + requirements.txt + README).
  2. Corre el build de tu SO:
       Linux/macOS:  ./build/build_portable.sh portable ./requirements.txt
       Windows:      powershell -ExecutionPolicy Bypass -File build\build_portable.ps1 portable .\requirements.txt
  3. Compila el sat-launcher (Go) para tu SO/arch, cópialo a portable/ + agrega notebooks/, RE-EMPACA.
  4. Verifica: el tarball se extrae en OTRA ruta y abre JupyterLab SIN token (localhost:8888/lab directo).
  5. (si publicas tú)  hf upload del tarball; REPORTA sha256 + oid LFS.
     (si no)            envíame el tarball + sha256 y yo lo publico.
CONSTRAINTS: nunca imprimas/commitees tokens; usa ruta SIN espacios; chmod +x en Unix; macOS firma ad-hoc
             y quita com.apple.quarantine. Pesado en disco externo si hace falta.
REPORTA: sha256 del tarball, versión de Python, y la salida de la PRUEBA DE RELOCALIZACIÓN.
```

---

## "Otros escenarios" (cuando haga falta compilar más)
Estas instrucciones son **copiables y parametrizables**. Cuando necesites un binario/tarball nuevo, adaptas
y **re-despachas** al agente remoto del SO correspondiente:

| Escenario nuevo | Qué tocas |
|---|---|
| Otra **arquitectura** (linux-arm64, macos-x86_64) | ya contemplado en `build_portable.sh` (detecta el triple); despacha al agente de esa arch |
| Otra **versión de Python** | `PY_VERSION` / `PBS_TAG` en los scripts |
| **Bibliotecas extra** o distinta versión | edita `requirements.txt` (respeta `numpy<2`) y re-despacha |
| **Otro tipo de proyecto** (no satelital) | mismo molde: otro `requirements.txt` + otros `notebooks/` |
| Nuevo **SO** | añade su rama de detección + su agente remoto |

> La idea de fondo: el **líder no necesita todas las máquinas**, necesita el **protocolo**. Mandas una copia
> de `build/` + el contexto, recibes el `sha`, registras en el manifest y verificas. Escala sin saturarte.
