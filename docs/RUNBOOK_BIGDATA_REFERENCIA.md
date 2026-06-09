# Publicar y actualizar el laboratorio (runbook de releases)

**Autor:** Dr. Abel Coronado (@abxda)
**Qué es esto:** la rutina operativa —paso a paso— para **publicar y actualizar**
cada pieza del laboratorio de Big Data: los binarios en **GitHub Releases**, los
artefactos pesados y el **manifiesto** en **Hugging Face**, el mecanismo de
**auto-actualización** del launcher Portable y los **cuadernos** del curso.

Complementa a [`DEPLOY_MULTIOS.md`](DEPLOY_MULTIOS.md): aquél explica *la
arquitectura* (qué es cada pieza y por qué); **éste explica cómo se publica y
actualiza cada una sin romper nada**.

> **Las 4 reglas de oro** (aplican a todo lo de abajo)
> 1. **NUNCA** imprimas, ecoes ni commitees `.hf_token` ni `.github_token`. Léelos
>    al vuelo (`export HF_TOKEN=$(cat /d/BDP/.hf_token)`) y nada más.
> 2. **Sube primero el artefacto, verifica su `oid` en HF, y SÓLO DESPUÉS edita el
>    manifiesto.** Así nunca hay una ventana donde el manifiesto apunte a un SHA
>    que aún no existe (el alumno abortaría con "SHA no coincide").
> 3. **Editar el manifiesto es additivo y anti-clobber**: bajar fresco → editar
>    una sola clave → `diff` (debe cambiar SOLO esa línea) → subir → re-bajar y
>    confirmar **COINCIDE**.
> 4. **Verifica integridad de punta a punta**: `sha256` local == `oid` LFS remoto
>    == valor en el manifiesto/descriptor. Si los tres no coinciden, no terminaste.

---

## 0. Mapa del dataset Hugging Face `abxda/bdp-lab`

Todo lo pesado vive aquí. El meta-launcher lee `…/resolve/main/manifest.txt`.

```
abxda/bdp-lab  (dataset)
├── manifest.txt                              ← índice maestro (texto; lo lee el meta-launcher)
│
├── meta-launcher-windows-amd64.exe           ← copia del meta-launcher (Win) — fuente: GitHub Release
├── meta-launcher.com                         ← copia del meta-launcher (Linux/macOS, Cosmopolitan)
│
├── bdp-portable-windows-amd64.tar.gz   2.7GB ← STACK Portable Win  (incluye el exe SEMILLA del launcher)
├── bdp-portable-macos-arm64.tar.gz     2.5GB ← STACK Portable Apple Silicon
│
├── bdp-vagrant-windows-amd64.tar.gz    4.6MB ← SOLO el panel Vagrant (la caja se baja con `vagrant up`)
├── bdp-vagrant-linux-amd64.tar.gz      3.9MB ←  ”
│
├── bdp-container-launcher-windows-amd64.tar.gz  4.5MB ← SOLO el panel Container (la imagen se baja aparte)
├── bdp-container-launcher-linux-amd64.tar.gz    3.9MB
├── bdp-container-launcher-macos-arm64.tar.gz    3.8MB
├── bdp-container-linux-amd64.tar.gz     2.2GB ← IMAGEN Podman (Win usa ésta vía WSL2)
├── bdp-container-macos-arm64.tar.gz     2.1GB ← IMAGEN Podman (Apple Silicon)
│
├── launchers/                                ← AUTO-ACTUALIZACIÓN del launcher Portable (barato)
│   ├── bdpv6-launcher-windows-amd64.exe          11.9MB ← exe nuevo que se autodescarga
│   └── bdpv6-launcher-latest-windows-amd64.json  206B   ← descriptor {version, sha256, url}
│
├── cuadernos/                                ← ejercicios del curso (2 variantes por Spark)
│   ├── semana_2/{container,portable}/TestGlobalBigData.ipynb
│   ├── semana_3/{container,portable}/01_ProcesarDirecciones.ipynb …
│   └── semana_4/{container,portable}/01_…_KMeans.ipynb … 05_api.py 06_kafka_streaming.ipynb
│
└── datos/                                    ← datasets de apoyo
```

**Por qué dos variantes de cuaderno:** el stack NO es idéntico en las 3 soluciones
(ver `DEPLOY_MULTIOS.md` §7): **Container y Vagrant corren Spark 4.0 / Scala 2.13**
→ variante `container/`; **Portable corre Spark 3.4 / Scala 2.12** → variante
`portable/`. Cada launcher baja la variante correcta para su entorno.

---

## 1. ¿Qué tan caro es cada cambio? (decide la ruta antes de tocar nada)

| Cambio | Artefacto que se republica | Tamaño | Cómo llega al alumno | Ruta |
|---|---|---|---|---|
| Lógica/UI del **launcher Portable** | `launchers/…exe` + descriptor | **~12 MB** | **auto-update** al abrir | **§3** |
| Lógica/UI del **panel Vagrant/Container** | su `…-launcher-….tar.gz` | **~4–5 MB** | el meta-launcher baja el tarball nuevo | **§4** |
| **Stack pesado** Portable/Container (Spark, Java, libs) | el `.tar.gz` de 2–2.7 GB | **GB** | descarga completa la próxima vez | **§5** |
| **Cuaderno** del curso | el `.ipynb` en `cuadernos/…` | **KB** | botón "Cuaderno de prueba" / kit | **§6** |
| **Meta-launcher** (binario de entrada) | GitHub Release + copia HF | **~9 MB** | el alumno baja `releases/latest` | **§7** |

> **Regla de costo:** el exe del launcher Portable vive *dentro* del `.tar.gz` de
> 2.7 GB sólo como **semilla**. Una mejora del launcher se publica por **§3 (12 MB)**,
> NO re-subiendo los 2.7 GB. Los 2–2.7 GB sólo se republican (§5) si cambia el
> **stack** (versión de Spark, una librería del entorno, etc.).

---

## 2. Preparación común (toolchain)

```bash
# Wails + Go (paneles). En Windows: CGO vía MSYS2 gcc.
export PATH="$PATH:/c/Users/abel.coronado/go/bin:/c/msys64/ucrt64/bin:/c/msys64/mingw64/bin"
export CGO_ENABLED=1
go version            # >= 1.23
wails version         # v2.12.x

# Token HF (sólo para SUBIR; léelo al vuelo, NUNCA lo ecoes)
export HF_TOKEN=$(cat /d/BDP/.hf_token)
# 'hf' es el CLI de huggingface_hub:  pip install -U huggingface_hub
```

Helper que repetiremos para **verificar el `oid` LFS** de un archivo en HF:

```bash
oid() {  # uso: oid <ruta-en-el-dataset>
  curl -s "https://huggingface.co/api/datasets/abxda/bdp-lab/tree/main/$(dirname $1)" \
   | python -c "import sys,json,os;p='$1';[print(f.get('lfs',{}).get('oid','')[:16]) for f in json.load(sys.stdin) if f['path']==p]"
}
```

---

## 3. Actualizar el launcher Portable (AUTO-UPDATE, ~12 MB) — la ruta barata

El launcher Portable (`bdpv6-launcher`) trae **auto-actualización** (`selfupdate.go`).
Al arrancar, **antes de levantar servicios**, consulta el descriptor en HF; si hay
una versión mayor, **baja el exe (~12 MB), verifica su SHA-256, se reemplaza a sí
mismo y se relanza**. El stack de 2.7 GB **no se vuelve a bajar nunca**.

### 3.1 Sube la versión EN EL CÓDIGO (obligatorio)
`app.go` → `var AppVersion = "X.Y.Z"`. El descriptor que publiques **debe declarar
exactamente esa misma versión**; si no, el alumno no actualiza (o entra en bucle).

```go
// D:\BDP\BDPV6_launcher\app.go
var AppVersion = "0.2.2"   // súbela en cada release
```

### 3.2 Compila el exe
```bash
cd /d/BDP/BDPV6_launcher
wails build -platform windows/amd64 -skipbindings
# -> build/bin/bdpv6-launcher.exe
SHA=$(sha256sum build/bin/bdpv6-launcher.exe | cut -d' ' -f1); echo $SHA
```

### 3.3 Sube el exe a `launchers/` y verifica el `oid`
```bash
hf upload abxda/bdp-lab build/bin/bdpv6-launcher.exe \
   launchers/bdpv6-launcher-windows-amd64.exe \
   --repo-type dataset --commit-message "launcher portable vX.Y.Z (+ cambios)"
oid launchers/bdpv6-launcher-windows-amd64.exe     # debe == $SHA
```

### 3.4 Actualiza el descriptor (version + sha256 nuevos) y confirma COINCIDE
```bash
cat > latest.json <<EOF
{"version":"0.2.2","sha256":"$SHA","url":"https://huggingface.co/datasets/abxda/bdp-lab/resolve/main/launchers/bdpv6-launcher-windows-amd64.exe"}
EOF
hf upload abxda/bdp-lab latest.json launchers/bdpv6-launcher-latest-windows-amd64.json \
   --repo-type dataset --commit-message "self-update descriptor -> vX.Y.Z"
curl -fsSL "https://huggingface.co/datasets/abxda/bdp-lab/resolve/main/launchers/bdpv6-launcher-latest-windows-amd64.json" -o r.json
diff -q latest.json r.json && echo "DESCRIPTOR COINCIDE ✅"
rm -f latest.json r.json
```

### 3.5 Comprobación no-GUI (que el alumno no abortará por hash)
```bash
curl -fsSL "https://huggingface.co/datasets/abxda/bdp-lab/resolve/main/launchers/bdpv6-launcher-windows-amd64.exe" -o pub.exe
[ "$(sha256sum pub.exe|cut -d' ' -f1)" = "$SHA" ] && echo "SELF-UPDATE OK ✅"; rm -f pub.exe
```

> **Orden importa:** primero el exe (§3.3), luego el descriptor (§3.4). Si publicas
> el descriptor apuntando a un SHA que aún no existe, todo alumno que abra en esa
> ventana aborta la actualización.
>
> **Pendiente conocido:** hoy sólo existe descriptor/exe para **windows-amd64**.
> Para dar auto-update al Portable de **Apple Silicon** hay que publicar
> `launchers/bdpv6-launcher-{macos-arm64.exe, latest-macos-arm64.json}` (el código
> ya arma la URL con `runtime.GOOS/GOARCH`).

---

## 4. Actualizar un panel chico (Vagrant / Container) — tarball ~5 MB

Estos paneles **no** tienen auto-update: el meta-launcher baja el tarball, verifica
el SHA del **manifiesto** y lanza. Por eso aquí **sí** se toca el manifiesto.

```bash
cd /d/BDP/vagrant-onboarding-panel        # (o container-onboarding-panel)
wails build -platform windows/amd64 -skipbindings

# Re-empaca el tarball CONSERVANDO su estructura (mismos archivos en la raíz).
mkdir stage && cd stage
curl -fsSL "https://huggingface.co/datasets/abxda/bdp-lab/resolve/main/bdp-vagrant-windows-amd64.tar.gz" -o ../cur.tgz
tar -xzf ../cur.tgz README.md                              # conserva lo que no sea el exe
cp ../../build/bin/vagrant-onboarding-panel.exe .
tar -czf ../new.tgz README.md vagrant-onboarding-panel.exe
cd ..; SHA=$(sha256sum new.tgz | cut -d' ' -f1); echo $SHA

# Sube y verifica oid ANTES de tocar el manifiesto
hf upload abxda/bdp-lab new.tgz bdp-vagrant-windows-amd64.tar.gz \
   --repo-type dataset --commit-message "panel Vagrant: <cambio>"
oid bdp-vagrant-windows-amd64.tar.gz       # debe == $SHA
```

Luego actualiza **sólo** la clave de SHA en el manifiesto (ver §8 para el ritual
anti-clobber). Clave a cambiar: `windows-amd64-vagrant.sha256=$SHA`.

> Container es idéntico pero con `bdp-container-launcher-<os>-<arch>.tar.gz` y la
> clave `<os>-<arch>-container.sha256`. La **imagen** Podman (2 GB) va por §5.

---

## 5. Actualizar un stack pesado (Portable / imagen Container) — GB

Sólo cuando cambia el **contenido del entorno** (versión de Spark, una librería,
el seed del launcher, etc.). Se republica el `.tar.gz` completo y se actualiza su
SHA en el manifiesto.

```bash
# Re-empaca el tarball del stack (ejemplo: cambiar SÓLO el exe-semilla dentro del
# Portable sin re-tarear toda la carpeta, vía tar --delete/--append):
cp bdp-portable-windows-amd64.tar.gz nuevo.tar.gz
tar --delete -f nuevo.tar.gz ./bdpv6-launcher.exe
( cd _staging && tar --append -f ../nuevo.tar.gz ./bdpv6-launcher.exe )
gzip ...   # (re-comprimir según cómo se generó el original)
SHA=$(sha256sum nuevo.tar.gz | cut -d' ' -f1)

hf upload abxda/bdp-lab nuevo.tar.gz bdp-portable-windows-amd64.tar.gz \
   --repo-type dataset --commit-message "stack portable: <cambio>"
oid bdp-portable-windows-amd64.tar.gz       # debe == $SHA
```

Después: manifiesto `windows-amd64-portable.sha256=$SHA` (ritual §8).
**Tiempo de subida real (referencia):** 2.55 GB ≈ **3 min** a ~14 MB/s.

> El **seed** del launcher dentro del Portable puede quedar atrás respecto a
> `launchers/` y no pasa nada: el alumno baja el stack viejo (seed vX) y en el
> primer arranque el **auto-update (§3)** lo lleva a la última versión. El seed
> sólo se refresca (§5) cuando ya toca republicar el stack por otra razón.

---

## 6. Actualizar cuadernos del curso (`cuadernos/…`) — KB

No tienen SHA en el manifiesto ni versión: se suben/regeneran y listo. **Respeta
las dos variantes** (`container/` Spark 4.0 y `portable/` Spark 3.4).

```bash
# Generador de variantes (mantiene COMMON + parches por perfil):
python /d/BDP/Curso_BDP/_build/build_notebooks.py        # crea ambas variantes

hf upload abxda/bdp-lab cuadernos/ cuadernos/ --repo-type dataset \
   --commit-message "cuadernos: <cambio>"                # sube la carpeta entera
```

Los botones **"Cuaderno de prueba"** de los 3 launchers bajan
`cuadernos/semana_2/<variante>/TestGlobalBigData.ipynb` — la variante correcta por
launcher (Container/Vagrant→`container/`, Portable→`portable/`).

---

## 7. Publicar el meta-launcher (GitHub Release + copia HF)

El meta-launcher es el **único** binario que el alumno baja a mano, desde
**GitHub Releases** (URLs `releases/latest/download/…`, no fijadas a versión).

```bash
cd /d/BDP/bdp-meta-launcher
# Compila por plataforma (Go puro, sin CGO) — ver AGENTS.md del repo:
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o meta-launcher-windows-amd64.exe .
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o meta-launcher-linux-amd64 .
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o meta-launcher-macos-arm64 .
GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -o meta-launcher-macos-amd64 .
./meta-launcher-<plataforma> --self-test     # "Auto-prueba CP2 EXITOSA"

# Publica el Release (gh CLI):
gh release create vX.Y.Z meta-launcher-* \
   --title "Meta-Launcher vX.Y.Z" --notes "<cambios>"

# (Opcional) copia al dataset HF para que todo viva en un lugar:
hf upload abxda/bdp-lab meta-launcher-windows-amd64.exe meta-launcher-windows-amd64.exe --repo-type dataset
```

> Las guías de descarga (`docs/ARRANQUE_*.md`) usan `releases/latest/download/…`,
> así que un parche **no rompe** los enlaces de los alumnos.

---

## 8. El ritual anti-clobber del manifiesto (paso a paso)

**Todo** cambio de SHA en el manifiesto se hace así, sin excepción:

```bash
cd /d/BDP/_manifest_work
curl -fsSL "https://huggingface.co/datasets/abxda/bdp-lab/resolve/main/manifest.txt" -o m.txt
cp m.txt m.orig

CLAVE="windows-amd64-vagrant.sha256"        # la clave EXACTA a cambiar
OLD=$(grep -oE "$CLAVE=[a-f0-9]+" m.txt | cut -d= -f2)
sed -i "s/$CLAVE=$OLD/$CLAVE=$SHA/" m.txt

diff m.orig m.txt                            # DEBE mostrar SÓLO esa línea
hf upload abxda/bdp-lab m.txt manifest.txt --repo-type dataset \
   --commit-message "SHA $CLAVE -> <motivo>"

curl -fsSL "https://huggingface.co/datasets/abxda/bdp-lab/resolve/main/manifest.txt" -o r.txt
diff -q m.txt r.txt && echo "HF COINCIDE ✅" || echo "DIFIEREN ❌"
rm -f m.txt m.orig r.txt
```

### Esquema del manifiesto (recordatorio)
Clave base `<os>-<arch>-<solucion>`:

| Sufijo | Significado | ¿Obligatorio? |
|---|---|---|
| `.file` | nombre del archivo destino | sí |
| `.sha256` | hash SHA-256 del archivo | sí |
| `.url` | URL absoluta de descarga | opcional |
| `.launch` | ejecutable de entrada | opcional → **ACTIVA** la solución |
| `.size` | etiqueta informativa de tamaño | opcional |

Para Container, además: `container-image-<arch>.{file,sha256,image,size}`.
**Publicar una plataforma nueva = añadir su `.launch`** (el meta-launcher la ofrece
sin recompilar).

---

## 9. Versionado y repos

| Pieza | Repo GitHub | Versión vive en | Se publica por |
|---|---|---|---|
| Meta-launcher | `abxda/bdp-meta-launcher` | tag del Release | §7 |
| Launcher Portable | `abxda/bdpv6-launcher` | `AppVersion` en `app.go` | §3 (auto-update) / §5 (stack) |
| Panel Vagrant | `abxda/vagrant-onboarding-panel` | `wails.json` info | §4 |
| Panel Container | `abxda/container-onboarding-panel` | `wails.json` info | §4 |
| Imagen Container | `abxda/quasar-container` | tag de imagen | §5 |

**Siempre** `git commit` + `git push` del código fuente tras compilar y publicar
(para no perder cambios). Cierra los commits con:
`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

## 10. Checklist de cada release

- [ ] Versión subida en el código (si aplica §3: `AppVersion` y el descriptor coinciden).
- [ ] Artefacto compilado y `sha256` local anotado.
- [ ] Artefacto subido a HF (o Release) **antes** de tocar el manifiesto.
- [ ] `oid` LFS remoto == `sha256` local.
- [ ] Manifiesto/descriptor editado anti-clobber → `diff` de una sola línea → COINCIDE.
- [ ] (Portable §3) comprobado que el exe publicado verifica contra el descriptor.
- [ ] Código fuente commiteado y **pusheado** a GitHub.
- [ ] Tokens nunca impresos ni commiteados.
