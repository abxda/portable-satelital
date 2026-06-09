# PortableSatelital — blueprint para el agente que lo construirá

> **Qué es esto:** el plano para forkear nuestra arquitectura *Portable Big Data* hacia un
> laboratorio **mínimo, portable y multi-SO** orientado a **segmentación de imágenes satelitales**
> (Shepherd segmentation + estadísticas zonales + clasificación), **sin** Hadoop/Spark/Kafka/Elasticsearch.
> Solo **JupyterLab + un stack geoespacial de Python**. Lo escribió el agente del proyecto Big Data para
> heredarle a otro agente toda la sabiduría ganada a golpes. Léelo entero antes de teclear nada.

---

## 0) TL;DR — el producto final

Un alumno baja **UN** binario (el *meta-launcher*), lo ejecuta, elige "Portable", y el sistema:
descarga (verifica SHA-256) → extrae → abre **JupyterLab** en el navegador, **ya con el token puesto**,
con todo el stack geoespacial listo. Cero `pip install`, cero `conda`, cero nube. Idéntico en
**Windows / macOS Apple Silicon / Linux**.

Es **la misma arquitectura** que el Big Data lab, pero **adelgazada**: no hay JVM, ni HDFS, ni servicios.
El "portable" es: **Python embebido + las wheels geoespaciales + JupyterLab + un launcher diminuto**.

---

## ⚡ Entorno YA disponible en ESTA máquina (Windows) — léelo primero, te ahorra trabajo

Corres en la máquina del Dr. (**Windows x64**). Esto ya está instalado y verificado (no lo re-descubras):

| Herramienta | Versión | Para qué |
|---|---|---|
| **Go** | **1.26.2** (windows/amd64, **CGO_ENABLED=0**) | compilar meta-launcher + sat-launcher (Go puro) |
| Wails | v2.12.0 | disponible; **NO** lo necesitas para el launcher mini |
| Python | 3.11.9 + pip 24.0 | construir el **portable de Windows** aquí mismo |
| hf CLI | 1.17.0 | crear el dataset HF + subir |
| gh CLI | 2.92.0 (logueado `abxda`, scope `repo`) | crear el repo de GitHub |
| git + SSH | 2.53 + `id_ed25519` (libre) | `git push` por SSH |
| tar / curl | GNU tar 1.35 / curl 8.18 | empaquetar / descargar |

### 🔑 Consecuencia ENORME: los binarios Go se cross-compilan AQUÍ (no necesitas las otras máquinas para ellos)
El meta-launcher y el sat-launcher son **Go puro (sin CGO)** → desde este Windows generas los binarios de los **3 SO**:
```bash
# (bash) sat-launcher / meta-launcher para los 3 SO sin salir de esta máquina:
GOOS=windows GOARCH=amd64 go build -o sat-launcher.exe
GOOS=linux   GOARCH=amd64 go build -o sat-launcher
GOOS=darwin  GOARCH=arm64 go build -o sat-launcher     # luego firma ad-hoc para Mac
# (PowerShell)  $env:GOOS='linux'; $env:GOARCH='amd64'; go build -o sat-launcher
```

### Lo ÚNICO que sí requiere agentes remotos: los Python portables de Linux y macOS
- **Windows** (binarios Go + **python portable**) → todo **aquí** (`build/build_portable.ps1`).
- **Linux / macOS** → el **python portable** trae wheels **binarias** (GDAL/numba) por SO → **agente remoto**
  en cada uno (`build/build_portable.sh`). Es lo único que NO se cross-buildea.

➡️ **Resumen:** aquí produces **los 3 binarios Go + el portable de Windows**. Despachas a remotos **solo** los
portables de Python de **Linux y macOS**. (Ver `build/README.md` para el protocolo con agentes remotos.)

### 📦 Esta carpeta es AUTOCONTENIDA y RELOCATABLE
Muévela a `D:\`, a un USB o a otra máquina: **todo sigue funcionando**. Los scripts se **auto-localizan**
(`build_portable.*` encuentran `requirements.txt` por la ubicación del propio script, sin importar el CWD);
las referencias internas (`credentials/`, `reference/`, `build/`, `requirements.txt`) son **relativas** y
viajan con la carpeta — **no hay rutas absolutas a `D:\BDP\...`**. Lo único que vive en el **sistema** (no en
la carpeta) es el **toolchain** (Go, Python, gh, ssh): debe existir en la máquina donde compiles. Si mueves
la carpeta a una máquina sin ese toolchain, instálalo allá o usa los agentes remotos.

---

## 1) Arquitectura (qué reutilizar y qué adelgazar)

```
  meta-launcher (1 binario por SO, Go puro)         <-- REUTILIZAR casi tal cual
        │  diagnostica SO/arch, lee manifest.txt de HF, baja+verifica(SHA-256)+extrae+lanza
        ▼
  portable-satelital-<os>-<arch>.tar.gz              <-- CONSTRUIR nuevo (lo pesado)
        ├── python/            (intérprete embebido + wheels del requirements.txt)
        ├── notebooks/         (el tutorial .ipynb + datos de ejemplo)
        └── sat-launcher(.exe) (launcher DIMINUTO: arranca jupyter, captura token, abre navegador)
        ▼
  HF dataset (abxda/portable-satelital)              <-- CREAR nuevo (con .hf_token)
        ├── manifest.txt                  (claves .sha256 + .url + .launch por solución/SO)
        ├── portable-satelital-*.tar.gz   (los tarballs por SO, LFS)
        └── launchers/                    (binario del sat-launcher + descriptor de self-update)
```

**Diferencia clave con Big Data:** allá el launcher era una app **Wails (GUI)** que orquestaba 5 servicios
(HDFS/Spark/Kafka/ES) con pestañas. Aquí **NO necesitas eso**. El `sat-launcher` es un **programa de
consola en Go puro (~150 líneas, sin CGO)** que solo: (1) resuelve el python del portable, (2) lanza
`python -m jupyterlab`, (3) **captura la URL con token** que Jupyter imprime, (4) abre el navegador en esa
URL, (5) queda corriendo y al cerrarse (Ctrl-C / cerrar ventana) detiene Jupyter. Cross-compila trivial.

---

## 2) El stack mínimo (Python, todo por pip)

| Dependencia | Rol | Nota |
|---|---|---|
| `jupyterlab` | entorno de notebooks | corre local |
| `numpy<2` | base numérica | **el ÚNICO pin que de verdad importa** (compat. con numba) |
| `scipy` | soporte numérico de `pyshepseg` | |
| `numba` | acelera la eliminación iterativa de Shepherd | trae LLVM en la wheel; fija el tope de numpy |
| `scikit-learn` | K-Means de Shepherd + clasificador final | |
| `pandas` | tabla de features, balanceo, CSV | |
| `pyshepseg` | Shepherd segmentation | puro Python; depende de numba+sklearn |
| `rasterio` | leer/escribir GeoTIFF + poligonizar | **trae su propio GDAL en la wheel** |
| `geopandas` | vector I/O, `.gpkg`, mapas | arrastra shapely (GEOS) y pyogrio (GDAL) en wheels |
| `exactextract` | estadísticas zonales por segmento | |
| `matplotlib` | visualizar mapas | |
| `pyyaml` | config YAML (opcional) | puro Python |

```bash
python -m venv .venv && source .venv/bin/activate
pip install jupyterlab "numpy<2" scipy numba scikit-learn pandas \
            pyshepseg rasterio geopandas exactextract matplotlib pyyaml
jupyter lab
```

Ver `requirements.txt` (incluido en esta carpeta).

**Tres reglas de oro del stack:**
1. **`numpy<2`** — sin este pin, pip rompe el stack por el lado de numba. Es el único pin crítico.
2. **NO agregues `gdal` suelto.** Lo consumes vía `rasterio`/`geopandas`, que ya lo empaquetan en sus wheels.
   Agregar GDAL aparte = conflicto de bibliotecas garantizado.
3. En Windows, si no quieres terminal, este mismo `requirements.txt` se carga en **JupyterLab Desktop**
   y te evitas el `venv`. (Pero para el portable multi-SO usaremos python embebido, ver §4.)

---

## 3) ⚠️ LA SABIDURÍA GANADA (lecciones que SÍ o SÍ vas a necesitar)

Estas las pagamos con sangre en el proyecto Big Data. Aplícalas o te van a morder igual:

1. **chmod +x en Unix tras extraer / auto-actualizar.** En macOS/Linux, bajar un binario con `os.Create`
   (modo 0644) o extraer un `.zip` por Finder DEJA el ejecutable SIN bit `+x` → "permission denied" al
   lanzar. El extractor y el self-update **deben** hacer `chmod 0755`. (En Windows el bit de ejecución no
   aplica.) Ver `reference/snippets/selfupdate.go` (el `os.Chmod` post-verificación).

2. **El self-update NO puede entregarse a sí mismo.** Si el binario desplegado tiene el bug del chmod, no
   puedes arreglarlo POR self-update (es el mecanismo roto). El fix tiene que llegar primero **en la
   semilla (tarball)**. Planea: una versión del tarball por SO trae el launcher arreglado; de ahí en
   adelante el self-update (binario ~pocos MB) es seguro.

3. **Espacios en la RUTA de instalación rompen todo.** Lo vimos en Hadoop (`'C:\Mi' no se reconoce`), pero
   aplica también a venvs/scripts de Python con rutas con espacios. **No actives el venv** (sus scripts
   hardcodean rutas y se parten con espacios). En su lugar: **invoca el python por ruta ABSOLUTA y pásale
   el entorno** (PATH, etc.). Y si la raíz del portable tiene espacios, **avisa claro** al usuario:
   "muévelo a una ruta corta sin espacios, ej. `C:\BDP-Sat`". Detéctalo con `strings.ContainsRune(root,' ')`.

4. **Jupyter SIN token — DECISIÓN TOMADA (el Dr. lo pidió explícito: el token da problemas).** En Big Data
   el token aleatorio fue una fuente de fricción: si el alumno teclea `localhost:8888` a mano, le sale
   pantalla de login. Para este mini-proyecto **desactivamos el token**. Es seguro porque Jupyter escucha
   **SOLO en 127.0.0.1** (local, no expuesto a la red). La forma robusta (independiente de versión) es
   **shippear un `jupyter_server_config.py`** dentro del portable con:
   ```python
   c.ServerApp.token = ''
   c.ServerApp.password = ''
   c.ServerApp.ip = '127.0.0.1'      # SOLO local — NO exponer a la red (token off + red abierta = mala idea)
   c.ServerApp.open_browser = False
   ```
   Así el launcher solo lanza `python -m jupyterlab` y **abre el navegador DIRECTO en
   `http://localhost:8888/lab`** — sin token, sin login, sin capturar nada. (El snippet
   `reference/snippets/jupyter_token.go` queda como **referencia del patrón de lanzamiento/entorno**, NO
   para capturar token.) **Mantén el bind en 127.0.0.1.**

5. **macOS Gatekeeper.** El alumno debe **bajar el meta-launcher con `curl`** (no por navegador, que lo
   marca como "archivo de texto" o lo pone en cuarentena). Comandos exactos para la doc del alumno:
   ```bash
   mkdir -p ~/SAT && cd ~/SAT
   curl -L -o meta-launcher-macos-arm64 "<URL del release>/meta-launcher-macos-arm64"
   chmod +x meta-launcher-macos-arm64
   ./meta-launcher-macos-arm64
   # si Gatekeeper bloquea: xattr -dr com.apple.quarantine ~/SAT && reintentar
   ```
   Firma ad-hoc el `.app`/binario y limpia `com.apple.quarantine` al empaquetar.

6. **Verifica SHA-256 de TODO lo que bajes** (tarballs y binarios). Disciplina del manifest: edición
   **aditiva por clave**, anti-clobber (baja el manifest → cambia UNA clave → diff de 1 línea → sube →
   re-baja → confirma que COINCIDE). Líneas en **LF** (no CRLF) y sha de 64 hex.

7. **Honestidad de "paso completado".** No marques un paso como exitoso solo por el exit code: en Windows
   un `.cmd`/proceso puede salir 0 aunque el trabajo real falle. Verifica el **resultado** (¿se creó el
   archivo esperado?) y escanea la salida por firmas de error. (En Big Data un formateo "completado" en
   realidad había tronado.)

8. **Relocatable de verdad.** El portable se extrae en una ruta DESCONOCIDA. Nada de rutas absolutas
   horneadas. El launcher resuelve todo a partir de su **propia ubicación** (`os.Executable()` →
   `filepath.Dir`). Ver `reference/snippets/paths.go` (patrón `Detect()` + `unwrapAppBundle` para el
   `.app` de Mac). Para Python embebido: el python NO debe depender de un venv activado.

9. **GDAL/PROJ data.** Si ves `PROJ: proj_create: Cannot find proj.db` o errores de GDAL al usar
   `pyproj`/`rasterio`/`geopandas`, exporta en el entorno del launcher `PROJ_DATA` (o `PROJ_LIB`) y
   `GDAL_DATA` apuntando a los datos que traen las wheels (dentro de `site-packages/pyproj/proj_dir/share/proj`
   y `.../rasterio/gdal_data`). Normalmente las wheels lo resuelven solas, pero tenlo a la mano.

---

## 4) Cómo construir el PORTABLE de Python por SO (la parte MÁS DIFÍCIL — receta concreta)

Objetivo: un `python/` autocontenido y **relocatable** (corre tras moverlo a una ruta desconocida), sin
tocar el sistema del alumno. Aquí están los baches reales donde fallan los intentos de "python portable":

### Regla base: usa **python-build-standalone**, NO un venv ni el "embeddable"
- Un **venv normal NO es relocatable**: hornea rutas absolutas en `pyvenv.cfg` (`home=...`) y en los
  *shebangs* de los scripts de consola (`jupyter`, `jupyter-lab`). Si mueves la carpeta, esos scripts truenan.
- El **"Windows embeddable"** oficial viene **SIN pip** y con `site` desactivado (`pythonXY._pth`): meter
  wheels ahí es un viacrucis.
- ✅ Usa **python-build-standalone** (CPython portable, de astral/indygreg `python-build-standalone`):
  trae pip, es self-contained y está pensado para moverse. Hay builds para `win-x64`, `linux-x64` y
  `macos-arm64`.

### La receta (corre EN cada SO/arch — NO se puede cross-buildear)
`pip` baja wheels **binarias específicas de la plataforma** (el GDAL de rasterio, el LLVM de numba…). **NO
puedes hornear el portable de Mac desde Windows.** Necesitas correr en cada SO (o CI), igual que el Big Data
lab necesitó agentes Mac/Linux.

```bash
# 1) Baja y extrae el standalone CPython del SO/arch -> portable/python/
#    (win: python.exe en la raíz ; linux/mac: bin/python3)

# 2) Instala las wheels DENTRO del portable, SIN activar nada:
portable/python/bin/python3 -m pip install --upgrade pip
portable/python/bin/python3 -m pip install -r requirements.txt
#    (Windows:  portable\python\python.exe -m pip install -r requirements.txt)

# 3) Unix: chmod +x al intérprete
chmod -R +x portable/python/bin

# 4) PRUEBA DE RELOCALIZACIÓN (lo que MÁS falla): MUEVE la carpeta y verifica
mv portable /tmp/destino-cualquiera
/tmp/destino-cualquiera/python/bin/python3 -m jupyterlab --version
/tmp/destino-cualquiera/python/bin/python3 -c "import rasterio, geopandas, numba, pyshepseg, exactextract, sklearn; \
    print('GDAL', rasterio.__gdal_version__); print('OK tras mover')"
```

- **Lanza Jupyter por MÓDULO** (`python -m jupyterlab`), nunca el script `jupyter-lab` (shebang con ruta vieja).
- **`numpy<2`** viene del requirements (único pin que rompe el stack por numba). **No agregues `gdal` suelto.**
- **GDAL/PROJ:** la prueba del paso 4 idealmente **LEE un GeoTIFF real**, no solo importa. Si truena con
  `proj.db not found` / GDAL data, exporta `PROJ_DATA`/`GDAL_DATA` (a los datos dentro de
  `site-packages/pyproj/...` y `.../rasterio/gdal_data`) en el entorno del launcher.
- **macOS:** firma ad-hoc el árbol, quita `com.apple.quarantine`, y **preserva symlinks + bits +x** al
  empaquetar el `.tar.gz` (si no, Gatekeeper/permisos te muerden).

### Empaquetar y publicar
`portable-satelital-<os>-<arch>.tar.gz` (preservando +x y symlinks en Unix) → `sha256` → `hf upload` (LFS) →
registra la clave en `manifest.txt` (anti-clobber). **Tamaño esperado:** ~600 MB–1.5 GB por SO (GDAL + LLVM
+ scipy pesan); mucho menos que los 2.5 GB del Big Data lab, pero no es "diminuto".

---

## 5) El sat-launcher diminuto (Go puro, sin CGO)

Responsabilidad única: abrir Jupyter sin fricción. Esqueleto (adáptalo; reusa el regex de
`reference/snippets/jupyter_token.go`):

```go
// 1) root := filepath.Dir(os.Executable())           // relocatable
// 2) py := filepath.Join(root, "python", pyBin())     // python.exe / bin/python3
// 3) avisa si strings.ContainsRune(root, ' ')         // ruta con espacios
// 4) cmd := exec.Command(py, "-m", "jupyterlab",
//        "--notebook-dir="+filepath.Join(root,"notebooks"), "--no-browser")
//    cmd.Env = env(root)   // PATH+=python ; JUPYTER_CONFIG_DIR -> carpeta con el jupyter_server_config.py
//                          // (token=''), bind 127.0.0.1 ; PROJ_DATA/GDAL_DATA si hace falta
// 5) espera a que el puerto 8888 responda (poll TCP) y abre el navegador DIRECTO en
//    http://localhost:8888/lab   (sin token: el config ya lo desactivó)
//    open (mac) / xdg-open (linux) / rundll32 url.dll,FileProtocolHandler (win)
// 6) queda corriendo -> al cerrarse (Ctrl-C / señal) mata el proceso de jupyter
```
> Sin token = el launcher es aún más simple: **no captura nada**, solo espera el puerto y abre `/lab`.
> Asegúrate de que Jupyter use el `jupyter_server_config.py` del portable (vía `JUPYTER_CONFIG_DIR`
> apuntando a una carpeta del portable, o `--config <ruta>`).

Cross-compila los 3:
```bash
GOOS=windows GOARCH=amd64 go build -o sat-launcher.exe
GOOS=linux   GOARCH=amd64 go build -o sat-launcher
GOOS=darwin  GOARCH=arm64 go build -o sat-launcher     # firma ad-hoc después
```

Self-update (opcional, recomendado): copia el patrón de `reference/snippets/selfupdate.go`
(descriptor JSON por SO + binario + verificación SHA-256 + **chmod 0755 en no-Windows** + swap + relaunch).

---

## 6) El meta-launcher multi-SO (reutilizar el template)

Está copiado en `reference/meta-launcher/` (Go puro, única dependencia `golang.org/x/sys`). Para forkearlo:

1. **Branding:** `internal/brand/brand.go` (nombre "Portable Satelital", colores, textos).
2. **HF repo:** apunta el fetch al nuevo dataset (`abxda/portable-satelital`) — busca la base URL del manifest.
3. **Lista de soluciones por SO:** `internal/platform/platform.go`. Aquí solo habrá **una** solución
   ("Portable"); quita Vagrant/Container. *(Lección Big Data: el meta-launcher tiene una LISTA hardcodeada
   por SO ADEMÁS del manifest; agregar/quitar solución = tocar esa lista **y** el manifest, y re-release.)*
4. **Cross-compila** los 3 binarios (`GOOS/GOARCH`), súbelos a un **GitHub Release** del repo nuevo.

El meta-launcher ya sabe: diagnosticar SO/arch, bajar el tarball, verificar SHA-256, extraer, y lanzar el
`.launch` indicado en el manifest (aquí, `sat-launcher`). No reinventes eso.

---

## 7) Credenciales y publicación (lee `credentials/README_CREDENCIALES.md`)

- **Hugging Face:** `credentials/.hf_token` (copiado). Sirve para **crear el dataset NUEVO** y **subir**.
  **TÚ (el agente nuevo) creas el repo HF**, dentro de la **misma cuenta `abxda`** (el agente Big Data NO lo
  creó, a propósito — es tu decisión y la del Dr.): `hf repo create abxda/portable-satelital --repo-type dataset`
  (o se auto-crea al primer `hf upload`). Ahí van manifest + tarballs + launchers. *(La estructura final y
  varias decisiones las afinará el Dr. CONTIGO más adelante — esto es el punto de partida, no la última palabra.)*
- **GitHub — la llave SSH está LIBRE y lista en esta máquina.** No hay archivo de token; el acceso es:
  (a) **llave SSH** `~/.ssh/id_ed25519` (su pública YA está registrada en GitHub para la cuenta `abxda`) →
  úsala para `git push` clonando/empujando con URL **`git@github.com:abxda/<repo>.git`**; y (b) **`gh` CLI ya
  logueado** (cuenta `abxda`, scope `repo`). → **SÍ puedes crear el repo** con
  `gh repo create abxda/portable-satelital --public` y empujar por SSH. **Respuesta a la duda del Dr.:** sí,
  el "git" sirve para crear el proyecto (vía `gh repo create`, scope `repo`); el push va por SSH, que está
  **libre**. Uso típico:
  ```bash
  git init && git add -A && git commit -m "init portable-satelital"
  git branch -M main
  git remote add origin git@github.com:abxda/portable-satelital.git
  GIT_SSH_COMMAND='ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15' git push -u origin main
  ```
  Si corres en OTRA máquina: `gh auth login` (o un PAT con scope `repo`) + registra la llave SSH.
- **Runbook:** `reference/PUBLICAR_Y_ACTUALIZAR.md` (rutina HF + GitHub del proyecto original; adáptala).

### 🔒 SEGURIDAD (no negociable)
- **NUNCA** imprimas, hagas echo, ni **commitees** `.hf_token` (ni ningún token). El `.gitignore` de esta
  carpeta ya los excluye — no lo quites.
- Los pushes con: `GIT_SSH_COMMAND='ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15'`.
- Termina los commits con: `Co-Authored-By: Claude <noreply@anthropic.com>` (o tu firma).
- Disciplina del manifest: aditiva, anti-clobber, verifica COINCIDE tras subir.

---

## 8) Qué hay en esta carpeta

```
PortableSatelital/
├── README.md                          <- este blueprint
├── requirements.txt                   <- el stack geoespacial (con numpy<2)
├── .gitignore                         <- excluye tokens / venv / build
├── build/
│   ├── build_portable.sh              <- construye el python portable (Linux/macOS), auto-localizable
│   ├── build_portable.ps1             <- construye el python portable (Windows), auto-localizable
│   └── README.md                      <- uso de los scripts + modelo de agentes remotos
├── credentials/
│   ├── .hf_token                      <- token HF (copiado; NO commitear)
│   └── README_CREDENCIALES.md         <- cómo usar HF + GitHub + seguridad
└── reference/                         <- plantillas reutilizables del proyecto Big Data
    ├── meta-launcher/                 <- fuente Go del meta-launcher (forkéalo)
    ├── snippets/
    │   ├── jupyter_token.go           <- patrón de lanzamiento/entorno de Jupyter (referencia)
    │   ├── selfupdate.go              <- self-update + el fix de chmod en Unix
    │   └── paths.go                   <- Detect() relocatable + unwrap .app de Mac
    └── PUBLICAR_Y_ACTUALIZAR.md       <- runbook de releases (HF + GitHub)
```

---

## 9) Plan de ejecución sugerido (orden)

1. **Repos:** `gh repo create abxda/portable-satelital --public`; crea el **dataset HF** `abxda/portable-satelital`.
2. **Notebook tutorial:** arma el `.ipynb` del flujo (Shepherd seg → poligonizar → estadísticas zonales →
   features → clasificación → mapas) + un GeoTIFF pequeño de ejemplo. Pruébalo en un venv con el requirements.
3. **Portable por SO:** construye `python/` embebido + wheels, relocatable, probado movido de carpeta. Empaqueta.
4. **sat-launcher:** escríbelo (Go puro), cross-compila los 3, pruébalo lanzando un portable real (token+navegador).
5. **meta-launcher:** forkea el template (brand + HF repo + 1 sola solución), cross-compila, GitHub Release.
6. **Publica:** sube tarballs (sha), launchers + descriptores, y arma `manifest.txt` (anti-clobber). Verifica e2e
   en los 3 SO: bajar meta-launcher → Portable → abre Jupyter con el tutorial.
7. **Doc del alumno:** una guía corta multi-SO (reusa el patrón de "comandos exactos de Mac" + "errores comunes").

> Hazlo **mínimo**. No metas Big Data. Si dudas, la respuesta es "menos". Y prueba SIEMPRE moviendo el
> portable de carpeta antes de cantar victoria. Suerte — vas con ventaja: todo el dolor ya lo pagamos nosotros.
