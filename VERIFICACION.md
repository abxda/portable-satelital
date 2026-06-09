# Cómo verificar este software (guía para áreas de TI y seguridad)

**Producto:** Laboratorio Satelital Portable (`SatLab.exe`) — entorno educativo de análisis
de imágenes satelitales (JupyterLab + stack geoespacial de Python), usado en cursos para
servidores públicos.
**Autor:** Dr. Abel Coronado ([@abxda](https://github.com/abxda)).
**Código fuente:** <https://github.com/abxda/portable-satelital> (100% público y auditable).

Este documento explica, en orden de profundidad, cómo comprobar que lo que su personal
descargó es exactamente lo que se publicó, de dónde proviene cada pieza, y qué hace (y qué
NO hace) en el equipo.

---

## 1) Qué hace el software (resumen para evaluación de riesgo)

- `SatLab.exe` es una aplicación de escritorio (Windows, WebView2) **sin instalador**: no
  toca el registro, no requiere privilegios de administrador, no instala servicios ni
  drivers. Todo vive en la carpeta donde se coloca el `.exe`.
- Al primer uso descarga una distribución de Python portable (~1 GB) desde un repositorio
  público de Hugging Face, **verifica su integridad** (ver §3) y la extrae junto al `.exe`.
- Después lanza **JupyterLab amarrado exclusivamente a 127.0.0.1** (loopback). No abre
  puertos hacia la red local ni acepta conexiones externas. No expone servicios.
- Conexiones de red salientes del launcher: únicamente a `huggingface.co` (catálogo y
  descarga de la distribución) y las que el usuario haga desde sus cuadernos.
- Desinstalación = borrar la carpeta. No deja rastro fuera de ella (los datos de Jupyter
  también viven dentro de la carpeta del laboratorio).
- El binario **no está firmado con Authenticode** (proyecto educativo sin certificado
  comercial); SmartScreen mostrará el aviso estándar. La autenticidad se verifica con los
  mecanismos de abajo, que son criptográficamente más fuertes que la reputación de
  SmartScreen.

## 2) Verificar que el binario proviene del código fuente público (procedencia / SLSA)

Cada release de `SatLab.exe` se compila en **GitHub Actions** (no en una máquina personal)
y se publica con **artifact attestation** de GitHub: una prueba criptográfica de qué
repositorio, commit y workflow produjeron el binario.

```bash
# Requiere GitHub CLI (https://cli.github.com)
gh attestation verify SatLab.exe --repo abxda/portable-satelital
```

Salida esperada: `✓ Verification succeeded` con el commit y el workflow que lo construyó.
Esto garantiza que el exe corresponde al código fuente público, sin modificaciones.

## 3) Cadena de integridad de lo que se descarga en tiempo de ejecución

1. El launcher trae **embebida** la llave pública Ed25519 del proyecto (visible en el
   fuente: `satlab-launcher/internal/sign/sign.go`).
2. El catálogo (`manifest.txt`) que dice qué descargar viaja **firmado** (`manifest.txt.sig`).
   Si la firma no verifica, el launcher se niega a descargar. Un atacante que comprometiera
   el hosting no puede redirigir las descargas sin poseer la llave privada.
3. Cada distribución descargada se verifica contra su **SHA-256** declarado en el catálogo
   firmado; si no coincide, el archivo se elimina y la instalación aborta.
4. Las bibliotecas de Python dentro de la distribución se instalaron con
   `pip install --require-hashes` contra `requirements.lock` (en este repositorio): cada
   wheel de PyPI quedó verificada contra su hash en el momento de construcción.
5. El intérprete de Python proviene de
   [python-build-standalone](https://github.com/astral-sh/python-build-standalone)
   (proyecto de Astral, licencia PSF), verificado contra su `SHA256SUMS` oficial al
   construir. **No se usa Anaconda ni conda** (sin implicaciones de licencia comercial).

Hashes oficiales de cada release: publicados en la página de
[Releases](https://github.com/abxda/portable-satelital/releases) (`SHA256SUMS.txt`).
Verificación manual:

```powershell
Get-FileHash SatLab.exe -Algorithm SHA256   # comparar contra SHA256SUMS.txt del release
```

## 4) Análisis antivirus independiente

Cada release se somete a [VirusTotal](https://www.virustotal.com) antes de publicarse y el
enlace al reporte se incluye en las notas del release. Puede re-subirlo usted mismo: el
hash SHA-256 del archivo debe coincidir con el del reporte.

## 5) Resumen de la postura de seguridad

| Control | Mecanismo |
|---|---|
| Procedencia del binario | GitHub Actions + artifact attestation (verificable con `gh`) |
| Integridad del catálogo | Firma Ed25519, llave pública embebida en el binario |
| Integridad de descargas | SHA-256 obligatorio (falla = se borra y aborta) |
| Integridad del stack Python | `pip --require-hashes` + lock auditable en el repo |
| Origen del intérprete | python-build-standalone (Astral), verificado contra SHA256SUMS |
| Exposición de red | Jupyter solo en 127.0.0.1; sin servicios, sin puertos abiertos |
| Privilegios | Ninguno: corre como usuario estándar, sin instalador, sin registro |
| Reversibilidad | Borrar la carpeta elimina todo |

¿Dudas o reporte de vulnerabilidades? Abra un issue en el repositorio o escriba al autor.
