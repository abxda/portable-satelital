# 🛰️ Laboratorio Satelital Portable

Entorno **portable y autocontenido** para el curso de **análisis de imágenes satelitales**
(segmentación de Shepherd, estadísticas zonales y clasificación) dirigido a servidores
públicos. **JupyterLab + stack geoespacial de Python** en una sola carpeta: sin instalador,
sin privilegios de administrador, sin conda/Anaconda, sin nube.

**Autor:** Dr. Abel Coronado ([@abxda](https://github.com/abxda))

---

## Para el alumno: 3 pasos

1. **Descarga** `SatLab.exe` desde el último [Release](https://github.com/abxda/portable-satelital/releases/latest)
   y colócalo en una carpeta tuya (ideal: `C:\SatLab`).
2. **Doble clic.** Si Windows muestra el aviso azul de SmartScreen: *"Más información" →
   "Ejecutar de todas formas"* (el porqué del aviso y cómo verificar la autenticidad están
   en [VERIFICACION.md](VERIFICACION.md)).
3. En la ventana, pulsa **"⬇️ Instalar laboratorio"** (una sola vez, descarga ~335 MB) y
   después **"🚀 Abrir laboratorio"**. Se abre JupyterLab en tu navegador, ya con todo listo.

Para desinstalar: cierra la ventana y borra la carpeta. Eso es todo.

> 💡 Dentro de los cuadernos puedes instalar bibliotecas extra con `%pip install <paquete>`;
> quedan dentro de tu laboratorio portable, sin tocar el resto del equipo.

## Alternativas en línea (sin instalar nada)

[![Abrir en Colab](https://colab.research.google.com/assets/colab-badge.svg)](https://colab.research.google.com/github/abxda/portable-satelital/blob/main/colab/Taller_ML_Urbano_Colab.ipynb)

- **Google Colab** — el taller completo sobre Python real en la nube de Google (motor
  [`pyshepseg`](https://github.com/ubarsc/pyshepseg)):
  [`colab/Taller_ML_Urbano_Colab.ipynb`](colab/Taller_ML_Urbano_Colab.ipynb).
- **Navegador (WebAssembly)** — el mismo taller sin instalar nada, con `shepherd-wasm`:
  [curso en línea](https://abxda.github.io/portable-satelital/).
- **Teoría portable** — la página interactiva de teoría para bajar y abrir con doble clic
  (`Teoria_Portable.zip`, disponible desde el sitio del curso). Útil si tu red bloquea GitHub.

## Qué incluye el laboratorio

| Pieza | Detalle |
|---|---|
| Python | 3.11 ([python-build-standalone](https://github.com/astral-sh/python-build-standalone), licencia PSF) |
| JupyterLab | entorno de cuadernos, **solo local** (127.0.0.1, sin exposición a la red) |
| Geoespacial | `rasterio` (GDAL), `geopandas`, `pyproj`, `shapely`, `exactextract` |
| Segmentación | [`pyshepseg`](https://github.com/ubarsc/pyshepseg) (Shepherd), `numba`, `scikit-learn` |
| Ciencia de datos | `numpy`, `scipy`, `pandas`, `matplotlib` |

Las versiones exactas (con hash criptográfico de cada paquete) están en
[`requirements.lock`](requirements.lock).

## Seguridad y verificación

Este software se distribuye a instituciones públicas; la cadena completa de custodia es
auditable: código fuente público, binario compilado en GitHub Actions con **attestation
verificable**, catálogo de descargas **firmado (Ed25519)** con la llave pública embebida en
el binario, y verificación **SHA-256 obligatoria** de todo lo que se descarga.

➡️ **Guía completa para áreas de TI:** [VERIFICACION.md](VERIFICACION.md)

## Estructura del repositorio

```
├── satlab-launcher/        launcher visual (Go + Wails v2). UN solo exe: instala y abre
│   ├── internal/fetch/       descarga + SHA-256 + extracción segura (solo stdlib)
│   ├── internal/sign/        verificación Ed25519 del catálogo
│   └── cmd/                  satlab-keygen / satlab-sign (herramientas de firma)
├── build/                  scripts que construyen el Python portable por SO
├── notebooks/              cuadernos del curso (se copian al portable)
├── requirements.txt        stack declarado  →  requirements.lock (pines + hashes)
├── VERIFICACION.md         guía de verificación para áreas de TI
└── docs/                   documentación interna del proyecto
```

## Construir desde el código fuente

```powershell
# 1) El Python portable (Windows x64) — descarga, verifica, instala con hashes, prueba y empaca
powershell -ExecutionPolicy Bypass -File build\build_portable.ps1

# 2) El launcher
cd satlab-launcher
wails build -platform windows/amd64    # -> build/bin/SatLab.exe
```

Los artefactos pesados se publican en el dataset de Hugging Face
[`abxda/portable-satelital`](https://huggingface.co/datasets/abxda/portable-satelital)
(catálogo `manifest.txt` + firma).

## Plataformas

| Plataforma | Estado |
|---|---|
| Windows x64 | ✅ disponible |
| Linux x64 | 🔜 en preparación |
| macOS Apple Silicon | 🔜 en preparación |

## Licencia

Código del launcher y scripts: MIT. El stack de Python conserva las licencias de cada
proyecto (todas open source; sin componentes de Anaconda).
