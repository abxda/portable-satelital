# build/ — construir los Python portables + modelo de agentes remotos

## Qué hay aquí
- **`build_portable.ps1`** — Windows x64 (receta canónica)
- **`build_portable.sh`** — Linux / macOS (espejo; corre EN el SO destino)

Cada script: baja **python-build-standalone** y lo **verifica contra el SHA256SUMS oficial**,
instala `../requirements.lock` **dentro** del portable con `--require-hashes` (sin venv),
escribe la config de **Jupyter sin token** (bind 127.0.0.1), copia `../notebooks/`, corre la
**prueba de relocalización** (imports + GeoTIFF real + **pip funcional**: los alumnos usan
`%pip install` en clase), y empaqueta el tarball + `sha256`.

```bash
# Linux / macOS  (corre EN esa máquina)
chmod +x build/build_portable.sh
./build/build_portable.sh

# Windows  (corre EN Windows)
powershell -ExecutionPolicy Bypass -File build\build_portable.ps1
```

> **El tarball NO lleva launcher.** La arquitectura (decisión 2026-06-09) es **UN solo
> launcher visual** (`satlab-launcher/` → SatLab por plataforma) que descarga este tarball
> desde el catálogo firmado y lanza Jupyter. El tarball solo trae `python/`,
> `jupyter-config/` y `notebooks/` en la raíz del tar.

---

## ⚠️ Regla dura: el PYTHON portable se construye EN cada SO/arch (no se cross-buildea)
`pip` baja **wheels binarias específicas de la plataforma** (el GDAL de rasterio, el LLVM de
numba). Cada tarball se construye en su plataforma. El **launcher** (Go/Wails) sí se compila
por plataforma en CI o en la máquina del agente correspondiente (en macOS/Linux Wails
requiere su toolchain nativo).

### Nota del lock multiplataforma
`requirements.lock` se generó en **Windows**. pip ignora los pines que no aplican a tu
plataforma, pero si `--require-hashes` falla por una wheel sin hash registrado para tu SO,
**re-genera el lock localmente** (instrucciones dentro del .sh) y repórtalo al líder para
que quede auditado en el repo (`requirements-linux.lock`, etc.).

---

## 🧭 Modelo líder / agentes remotos

- **El líder** (quien tiene este repo + las credenciales) es dueño del **manifest firmado**:
  registra los sha (aditivo, **anti-clobber**), **re-firma** (`cmd/satlab-sign`) y verifica
  end-to-end. La llave privada `credentials/satlab_ed25519.key` NUNCA sale de su máquina.
- **Cada agente remoto** (Linux x64, macOS arm64) recibe una copia del repo, corre el
  `build_portable.sh` de su SO y **reporta**: sha256 del tarball, versión de Python y la
  salida de la prueba de relocalización. Si publica él mismo a HF, reporta también el `oid` LFS.

### Plantilla de prompt para un agente remoto
```
PARA: Agente <SO> (Linux x64 / macOS arm64).
OBJETIVO: construir el Python portable de PortableSatelital para tu SO/arch.
PASOS:
  1. Clona github.com/abxda/portable-satelital (o recibe la carpeta).
  2. ./build/build_portable.sh        # verifica, instala con hashes, prueba, empaca
  3. (macOS) firma ad-hoc + quita com.apple.quarantine (el script ya lo intenta).
  4. Verifica: extrae el tarball en OTRA ruta y corre
     python/bin/python3 -m jupyterlab --version  y  -m pip --version.
  5. REPORTA: sha256 del tarball, versión de Python, salida de la prueba de
     relocalización. NO edites el manifest: eso lo hace el líder (firma Ed25519).
CONSTRAINTS: nunca imprimas/commitees tokens; ruta SIN espacios; chmod +x en Unix.
```

### El ritual del manifest (lo hace SOLO el líder)
1. Sube el tarball a HF (`hf upload …`) y verifica `oid` LFS == sha256 local.
2. Baja el `manifest.txt` actual, agrega/edita **solo** las claves de esa plataforma
   (`<os>-<arch>-portable.{file,sha256,version,size}`), diff mínimo.
3. **Re-firma**: `go run ./cmd/satlab-sign credentials/satlab_ed25519.key manifest.txt`
4. Sube `manifest.txt` y `manifest.txt.sig`, re-baja ambos y confirma que COINCIDEN.

| Escenario nuevo | Qué tocas |
|---|---|
| Otra arquitectura | `build_portable.sh` ya detecta el triple; despacha al agente de esa arch |
| Otra versión de Python | `PY_VERSION` / `PBS_TAG` en ambos scripts |
| Bibliotecas extra | `requirements.txt` → re-generar `requirements.lock` → reconstruir |
| Notebooks nuevos | `notebooks/` se actualiza barato re-empacando el tarball (o como kit aparte) |
