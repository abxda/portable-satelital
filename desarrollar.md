# Especificación técnica — Reimplementación portable de *Shepherd Segmentation* (local → WASM)

**Versión:** 1.0 · **Estado:** para diseño de plan por el desarrollador
**Contexto:** pipeline de clasificación de imágenes satelitales (`geocrop` / cuaderno *Semana 14*), segmentación orientada a objetos.
**Objetivo del documento:** dar al desarrollador todo lo necesario —algoritmo, nombres correctos, referencias, entorno, criterios de validación— para que diseñe e implemente, primero, una versión de Shepherd segmentation que corra en un **entorno portable local (pip puro, sin Conda)** y, después, en **WebAssembly (navegador)**.

---

## 1. Objetivo y alcance

Construir una implementación de **Shepherd et al. (2019) — segmentación por eliminación iterativa** que:

1. **Preserve el algoritmo original** (no un sustituto tipo SLIC/Felzenszwalb): mismos tres pasos, mismos parámetros, salida comparable segmento a segmento contra `pyshepseg`.
2. **Corra en un entorno portable local** instalable solo con `pip`/`uv` (sin Conda, sin RSGISLib, sin formato KEA).
3. **Sea portable a WASM/Pyodide**, eliminando la única dependencia que no compila a WASM: **numba**.

**Fuera de alcance:** descarga/preparación de imágenes, generación de etiquetas, extracción de features y clasificación (ya resueltos en el cuaderno existente; aquí solo se especifica el bloque de segmentación y su ruta a WASM).

---

## 2. Estrategia en dos fases

| Fase | Entregable | Propósito |
|---|---|---|
| **A — Local portable** | Reimplementación puro-`numpy`/`scipy` de Shepherd, validada contra `pyshepseg` | Oráculo de correctitud; corre en cualquier máquina con `pip`. No usa numba. |
| **B — WASM** | Misma lógica corriendo en navegador (Pyodide), con el *hot loop* en un módulo Rust/WASM | Portabilidad máxima; rendimiento aceptable sin numba. |

La Fase A es **requisito previo** de la Fase B: la versión puro-Python es lenta pero correcta, y sirve para verificar bit a bit la versión Rust/WASM.

---

## 3. Contexto: dónde encaja la segmentación

Flujo completo del pipeline (la segmentación es el paso 2):

1. Leer GeoTIFF multibanda (geomediana Sentinel-2, 12 bandas).
2. **Segmentación Shepherd → ráster de IDs de segmento (`segimg`).**  ← *este documento*
3. Poligonizar `segimg` (`rasterio.features.shapes`) → GeoPackage de segmentos.
4. Ráster de etiquetas (verdad-terreno) alineado a la imagen.
5. Proporción de clase por segmento (`numpy.bincount`).
6. Estadísticas espectrales por segmento (por banda).
7. Tabla de features → CSV.
8. Clasificación (scikit-learn).
9. Predicción + mapa + GeoPackage clasificado.

La salida que **debe** producir el bloque de segmentación: un array 2D `int32`/`uint32` (`segimg`) donde cada píxel lleva el ID de su segmento y `0` = nulo, georreferenciado igual que la imagen de entrada.

---

## 4. Especificación del algoritmo *Shepherd Segmentation*

### 4.1 Referencias canónicas

- **Algoritmo (primario):** Shepherd, J.D.; Bunting, P.; Dymond, J.R. *Operational Large-Scale Segmentation of Imagery Based on Iterative Elimination.* **Remote Sensing 2019, 11(6), 658.** DOI: `10.3390/rs11060658`. (Acceso abierto.)
- **Marco GEOBIA / RAT y proceso por tiles:** Clewley, D.; Bunting, P.; Shepherd, J.; Gillingham, S.; Flood, N.; Dymond, J.; Lucas, R.; Armston, J.; Moghaddam, M. *A Python-Based Open Source System for GEOBIA Utilizing Raster Attribute Tables.* **Remote Sensing 2014, 6(7), 6111–6135.** DOI: `10.3390/rs6076111`.
- **Implementación de referencia (a replicar):** `pyshepseg` v2.0.5 — github.com/ubarsc/pyshepseg (módulo `pyshepseg.shepseg`).
- **Implementación canónica C++:** RSGISLib — `rsgislib.segmentation.shepherdseg.run_shepherd_segmentation` (rsgislib.org).

### 4.2 Implementaciones existentes y por qué NO sirven "puras"

| Implementación | Lenguaje base | Bloqueador para WASM |
|---|---|---|
| `pyshepseg` | Python | Núcleo (`clump`, `eliminateSmallSegments`, `findMergeSegment`) decorado con `@njit` de **numba** (JIT vía LLVM) — no existe en Pyodide. |
| RSGISLib | C++ | Motor C++ + dependencia GDAL/KEA — aún más lejos de WASM. |

**Conclusión:** no existe hoy una implementación puro-Python lista. Pero el algoritmo es corto y bien definido (sección 4.3) y se reimplementa con piezas que **sí** corren en Pyodide: `sklearn.cluster.KMeans`, `scipy.ndimage.label`, `numpy`/`scipy`.

### 4.3 Los tres pasos (con nombres reales de `pyshepseg.shepseg`)

Firma de referencia (verificada en v2.0.5):

```python
doShepherdSegmentation(
    img,                      # ndarray (nBandas, nFilas, nCols)
    numClusters=60,
    clusterSubsamplePcnt=1,   # % de píxeles submuestreados para el KMeans
    minSegmentSize=50,        # umbral de eliminación (px)
    maxSpectralDiff='auto',   # límite de fusión en unidades del espacio espectral
    imgNullVal=None,
    fourConnected=True,       # 4- u 8-conectividad para clumping y vecindad
    verbose=False,
    fixedKMeansInit=False,
    kmeansObj=None,
    spectDistPcntile=50,      # percentil para 'auto' de maxSpectralDiff
)
```

**Paso 1 — Clustering (semillas).**
KMeans (sklearn) sobre un submuestreo de los píxeles (`clusterSubsamplePcnt`), con `numClusters` centros. Luego cada píxel de la imagen se asigna al centro más cercano → imagen de clusters.
*Nota de fidelidad:* el K-means de RSGISLib no tiene diferencias documentadas frente al de sklearn; `sklearn.cluster.KMeans` es sustituto fiel del paso 1.

**Paso 2 — Clumping (`clump`).**
Etiquetado de **componentes conexos** de la imagen de clusters: píxeles adyacentes con el mismo cluster forman un *clump*. Conectividad 4 (default) u 8 (`fourConnected`). El valor nulo (`ignoreVal`) se ignora.
→ Equivalente puro: `scipy.ndimage.label` (con la estructura de conectividad apropiada), aplicado por cluster o con etiquetado global que respete fronteras de cluster.
Devuelve `clumpimg` (IDs por píxel) y el siguiente ID libre.

**Paso 3 — Eliminación iterativa (`eliminateSmallSegments`).**
*"Empieza por el más pequeño y fusiónalo con el vecino espectralmente más similar; repite para tamaños mayores."* Procesa los segmentos con tamaño `< minSegmentSize`, del más pequeño al mayor, y cada uno se fusiona con el segmento **vecino** cuya media espectral sea la más cercana en **distancia euclidiana** en el espacio de bandas (`findMergeSegment`), sujeto al límite `maxSpectralDiff`. Modifica `seg` in situ y repite hasta que no queden segmentos bajo el umbral.

Funciones auxiliares (a replicar):
- `buildSegmentSpectra(seg, img, maxSegId)` → sumas espectrales por segmento y banda (para medias).
- `makeSegSize(seg)` → conteo de píxeles por segmento (histograma).
- `makeSegmentLocations(...)` → coordenadas de píxeles por segmento (en numba es `numba.typed.Dict`; en puro Python usar listas/dicts o índices).
- `findMergeSegment(segId, segLoc, seg, segSize, spectSum, maxSpectralDiff, fourConnected)` → elige el vecino con **mínima distancia euclidiana** entre medias espectrales, dentro de `maxSpectralDiff`.
- `autoMaxSpectralDiff(km, maxSpectralDiff, distPcntile)` → si `'auto'`, usa la **mediana** (percentil `distPcntile`) de las distancias entre centros de cluster; si `None`, usa 10× la mayor distancia (efectivamente sin límite); si número, ese valor.

**Paso 4 — Reetiquetado (`relabelSegments`).**
Tras la eliminación quedan IDs sin usar; se renumeran para que sean **contiguos** desde `minSegId` (1). `0` = nulo.

### 4.4 Tabla de parámetros (fuente de verdad para la API portable)

| `pyshepseg` | RSGISLib equiv. | Descripción | Default | Valor usado en el cuaderno |
|---|---|---|---|---|
| `numClusters` | `numClusters` | nº de clusters K-means (semillas) | 60 | 60 |
| `minSegmentSize` | `minPxls` / `min_n_pxls` | tamaño mínimo de segmento (px) | 50 | 100 |
| `clusterSubsamplePcnt` | `sampling` | % submuestreo para K-means | 1 | — |
| `maxSpectralDiff` | `distThres` | límite de fusión (unidades espectrales) | `'auto'` | `'auto'` |
| `spectDistPcntile` | — | percentil para `'auto'` | 50 | — |
| `fourConnected` | — | 4- u 8-conectividad | `True` | `True` |
| `imgNullVal` | — | valor nulo de entrada | `None` | nodata del GeoTIFF |

### 4.5 Estructura de salida (`SegmentationResult`)

Atributos verificados a replicar: `segimg` (array de IDs), `kmeans` (objeto KMeans ajustado, reutilizable entre tiles), `maxSpectralDiff` (valor efectivo usado), `singlePixelsEliminated`, `smallSegmentsEliminated`.

### 4.6 Procesamiento por tiles (imágenes grandes)

Para escenas que no caben en memoria (la geomediana estatal: 10005×11086×12 ≈ 2.66 GB), `pyshepseg.tiling` implementa el patrón de Clewley et al. (2014):
- `fitSpectralClustersWholeFile(...)` ajusta el K-means **una sola vez** sobre un submuestreo de toda la imagen → clusters consistentes entre tiles.
- `doTiledShepherdSegmentation(...)` segmenta tiles **solapados** y los cose, garantizando IDs únicos y continuidad en fronteras.

La reimplementación debe conservar este esquema: **un K-means global, segmentación por tiles solapados, costura**. Es además la base de la estrategia de streaming en WASM (sección 6.4).

---

## 5. Fase A — Entorno portable local

### 5.1 Stack (pip puro, sin Conda)

| Paquete | Rol | Instalación |
|---|---|---|
| `numpy` (`<2` si se usa numba de referencia) | arrays | PyPI |
| `scipy` | `ndimage.label`, utilidades | PyPI |
| `scikit-learn` | KMeans (paso 1) | PyPI |
| `rasterio` | I/O GeoTIFF (trae GDAL en la wheel) | PyPI |
| `geopandas` | vector / GeoPackage (trae GEOS/PROJ) | PyPI |
| `pandas`, `matplotlib` | tabla, visualización | PyPI |
| `pyshepseg` | **referencia/oráculo** (no es dependencia de producción) | `git+https://github.com/ubarsc/pyshepseg.git` |
| `numba` | **solo para correr el oráculo** `pyshepseg` localmente | PyPI |

> **`pyshepseg` no está en PyPI** — solo conda-forge. Para pip se instala desde GitHub. En producción/WASM **no** se distribuye; se usa únicamente como oráculo de validación local.

### 5.2 Entorno reproducible

```bash
python -m venv .venv && source .venv/bin/activate     # o: uv venv && source .venv/bin/activate
pip install "numpy<2" scipy scikit-learn rasterio geopandas pandas matplotlib
pip install "git+https://github.com/ubarsc/pyshepseg.git"   # oráculo + numba
```

(Recomendado `uv` por su solver real; el único *pin* crítico es `numpy<2` por numba, y **solo** aplica mientras se use el oráculo.)

### 5.3 Implementación de referencia (oráculo) en puro `numpy`/`scipy`

Entregable A: módulo `shepherd_pure.py` con la misma firma que `doShepherdSegmentation`, implementando los 4 pasos **sin numba**:

- Paso 1: `sklearn.cluster.KMeans` + asignación vectorizada por `argmin` de distancias a centros.
- Paso 2: `scipy.ndimage.label` (clumping) respetando fronteras de cluster y `fourConnected`.
- Paso 3: eliminación iterativa en Python/`numpy` — grafo de adyacencia de regiones, cola por tamaño, fusión al vecino de mínima distancia euclidiana entre medias, con `maxSpectralDiff`. **Este es el paso difícil** (inherentemente secuencial; no vectoriza limpio) y el que dominará el tiempo.
- Paso 4: reetiquetado contiguo.

### 5.4 Criterios de validación (oráculo)

Sobre **tiles pequeños** (p. ej. 512×512 de la geomediana):
1. **Igualdad de etiquetas** salvo permutación de IDs: comparar `segimg` propio vs `pyshepseg` con métrica de coincidencia de particiones (p. ej. Adjusted Rand Index ≥ 0.99, idealmente 1.0).
2. **Mismo nº de segmentos** ±tolerancia mínima atribuible a desempates.
3. **Mismo tamaño mínimo respetado** (ningún segmento < `minSegmentSize`, salvo nulos).
4. Documentar diferencias de **orden de fusión / desempate** (Shepherd no fija desempates; pequeñas diferencias en fronteras son esperables y aceptables — ver Riesgos).

---

## 6. Fase B — WebAssembly (navegador)

### 6.1 El único bloqueador real: numba

numba depende de `llvmlite`/LLVM JIT (genera código máquina en runtime), **incompatible con el sandbox WASM**. No hay numba en Pyodide ni ruta de producción para tenerlo. Su AOT (`numba.pycc`) está deprecado. **Por eso la Fase A produce una versión sin numba**, y la Fase B acelera el *hot loop* con un módulo Rust/WASM (sección 6.3).

### 6.2 Qué YA está disponible en Pyodide (no hay que portarlo)

Versiones verificadas en **Pyodide 0.29.4** (todo el geo-stack ya compilado a WASM):

| Paquete | Versión en Pyodide |
|---|---|
| `numpy` | 2.4.3 |
| `scipy` | (incluido) |
| `scikit-learn` | 1.8.0 |
| `scikit-image` | 0.25.2 |
| `pandas` | (incluido) |
| `shapely` (GEOS) | 2.1.2 |
| `pyproj` (PROJ) | 3.7.2 |
| `geopandas` | 1.1.3 |
| `fiona` | 1.9.5 |
| `rasterio` (GDAL) | 1.5.0 |
| `matplotlib` | (incluido) |

→ Los pasos 1, 2, 4 de Shepherd y **todo el resto del pipeline** (poligonización, etiquetas, features, clasificación, mapa) corren en Pyodide sin trabajo extra. Nota: en WASM no hay numba, así que `numpy 2.x` es válido (el *pin* `<2` de la Fase A no aplica aquí).
> Advertencia: el binding crudo `from osgeo import gdal` tiene un bug de import conocido; acceder a GDAL **siempre vía** `rasterio`/`fiona`/`geopandas`.

### 6.3 Arquitectura recomendada: híbrida Pyodide + core Rust/WASM

- **Pyodide/JupyterLite** ejecuta el cuaderno completo (sitio estático, p. ej. GitHub Pages).
- El **paso 3 (eliminación iterativa)** —y opcionalmente el paso 2— se implementa en **Rust** con `ndarray`, se compila a un **módulo WASM** (vía `wasm-bindgen` o como *side module* Emscripten) y se carga desde Pyodide. Recupera velocidad casi nativa preservando el algoritmo exacto.
- Paso 1 (KMeans) permanece en `scikit-learn` dentro de Pyodide.
- **Importante (ABI):** el módulo Rust/WASM debe compilarse contra el **toolchain Emscripten exacto** que fija la versión de Pyodide usada (p. ej. la línea 0.27+ usa Python 3.13 / Emscripten 4.0.9; las wheels WASM deben llevar el platform tag `pyodide_*_wasm32` correspondiente). Fijar versión de Pyodide al inicio del proyecto.
- Referencias Rust reutilizables: crates `geo`, `ndarray`, `geozero`, e implementaciones de componentes conexos/superpíxeles (`fast-slic`, `simple_clustering`) como base — **ninguna implementa Shepherd**, el paso 3 se porta a mano.

Alternativas (para que el desarrollador las pondere en su plan):
- **Pure-numpy en Pyodide sin Rust:** menos código, pero el paso 3 puede ir 1–2 órdenes de magnitud más lento; **viable solo para AOI pequeño**.
- **JS/TS + WASM sin Python:** máximo rendimiento, máxima reescritura (segmentación y stats en Rust/WASM, ML vía ONNX Runtime Web, render vía deck.gl); se pierde el cuaderno.
- **Precómputo servidor + navegador solo inferencia/mapa:** mínimo esfuerzo de navegador, pero no cumple "todo en el navegador" para los pasos pesados.

### 6.4 Datos grandes en el navegador

WASM es wasm32 con techo de memoria lineal ~4 GB; la escena de 2.66 GB **no se carga entera**.
- Convertir la fuente **una vez** a **Cloud-Optimized GeoTIFF (COG)** con tiles internos (256×256 o 512×512) y overviews (`gdal_translate -co TILED=YES -co COPY_SRC_OVERVIEWS=YES -co COMPRESS=LZW`).
- En el navegador leer **ventanas/tiles** por *range requests* (`geotiff.js` `readRasters({window})`, o lecturas con ventana de rasterio).
- Reproducir el esquema por tiles de `pyshepseg.tiling` (K-means global + tiles solapados + costura) — sección 4.6.
- Persistir rásteres intermedios en **OPFS** (Pyodide `mountNativeFS`).
- **Memory64 (wasm64)** solo como último recurso: disponible en Chrome 133 / Firefox 134 (early 2025, parte de WebAssembly 3.0), tope de motor 16 GB, **no en Safari**, y con penalización de 10 %–2× por los *bounds checks*. Diseñar para wasm32 + tiles; no depender de Memory64.

### 6.5 Clasificación en WASM (paso 8 del pipeline)

`scikit-learn` 1.8.0 corre nativo en Pyodide → el pipeline (`StandardScaler` + `StackingEstimator(MLP)` + `ExtraTrees`) funciona tal cual. Opcional: exportar a **ONNX** (`sklearn-onnx`) y ejecutar con **ONNX Runtime Web**. Advertencia: el `StackingEstimator` *custom* no lo convierte `skl2onnx` de fábrica — registrar un converter o refactorizar a `sklearn.ensemble.StackingClassifier` (este sí soportado).

---

## 7. Plan por milestones (esfuerzo orientativo, 1 desarrollador experto)

| # | Milestone | Esfuerzo | Salida verificable |
|---|---|---|---|
| 0 | Pipeline en JupyterLite/Pyodide salvo segmentación (pasos 3–9 sobre tile pre-segmentado) | 1–2 sem | Cuaderno corre en navegador con segmentos dados |
| 1 | Stats zonales sin `exactextract` (numpy/`scipy.ndimage`) y validación | 1 sem | Stats == exactextract (tol. flotante) en tile |
| 2 | **Shepherd puro-numpy (oráculo)** validado vs `pyshepseg` | 2–4 sem | ARI ≥ 0.99 en tiles 512² |
| 3 | **Core Rust/WASM** del paso 3 (y 2) cargado en Pyodide | 4–8 sem | Igual a M2 en correctitud, ~nativo en velocidad |
| 4 | Streaming + tiles (COG, ventanas, OPFS, costura) | 3–5 sem | Escena completa procesable por tiles |
| 5 | Render + pulido (mapa; opcional deck.gl/lonboard) | 1–2 sem | Mapa interactivo de segmentos clasificados |

**Umbrales de decisión:** si el objetivo es solo AOI, detenerse en M2 (puro-numpy) puede bastar y M3 (Rust) es opcional. Si se requiere escena completa interactiva, M3 es obligatorio. Si Safari es requisito, no depender de Memory64/WebGPU.

---

## 8. Entregables

1. `shepherd_pure.py` — implementación puro-`numpy`/`scipy` (Fase A), misma firma que `doShepherdSegmentation`.
2. Suite de validación contra `pyshepseg` (tiles de referencia + métricas ARI/nº segmentos).
3. Crate Rust + módulo WASM del paso 3 (y 2), con build reproducible contra el Emscripten de la versión de Pyodide fijada.
4. Cuaderno JupyterLite que integra el core WASM en el pipeline completo.
5. Conversor COG + utilidades de tiles/OPFS/costura.
6. README con versiones fijadas (Pyodide, Emscripten, paquetes) y guía de build.

## 9. Criterios de aceptación

- **Correctitud:** segmentación reimplementada ≈ `pyshepseg` (ARI ≥ 0.99 en tiles de prueba; sin segmentos bajo `minSegmentSize`).
- **Portabilidad local:** instalación solo con `pip`/`uv`, sin Conda, en Linux/macOS/Windows.
- **Portabilidad WASM:** el cuaderno corre íntegro en navegador (Chrome/Firefox actuales) sobre un AOI, y por tiles sobre la escena completa.
- **Fidelidad de algoritmo:** se conservan los 3 pasos y los parámetros de la tabla 4.4 (no sustitutos tipo SLIC).

## 10. Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Paso 3 (eliminación) secuencial y lento en puro-Python | Core Rust/WASM (M3); aceptar puro-numpy solo para AOI |
| Desempates de fusión difieren de `pyshepseg` (Shepherd no los fija) | Aceptar diferencias menores en fronteras; validar por ARI, no por igualdad exacta de IDs |
| ABI Emscripten/Pyodide desalineado | Fijar versión de Pyodide desde el día 1; build contra su toolchain exacto |
| 2.66 GB no cabe en WASM | Diseñar por tiles + COG + OPFS desde M4; no asumir carga completa |
| `StackingEstimator` custom no exporta a ONNX | Usar sklearn nativo en Pyodide, o refactor a `StackingClassifier` |
| Safari sin Memory64/WebGPU | Diseñar wasm32 + tiles + ONNX WASM CPU |

## 11. Referencias

1. Shepherd, Bunting, Dymond (2019). *Operational Large-Scale Segmentation of Imagery Based on Iterative Elimination.* Remote Sensing 11(6):658. DOI 10.3390/rs11060658.
2. Clewley et al. (2014). *A Python-Based Open Source System for GEOBIA Utilizing Raster Attribute Tables.* Remote Sensing 6(7):6111–6135. DOI 10.3390/rs6076111.
3. `pyshepseg` v2.0.5 — github.com/ubarsc/pyshepseg (módulo `shepseg`, `tiling`).
4. RSGISLib — `rsgislib.segmentation.shepherdseg` — rsgislib.org.
5. Pyodide 0.29.4 — lista de paquetes y changelog — pyodide.org.
6. WebAssembly 3.0 / Memory64 — webassembly.org; caniuse "wasm64".
7. `sklearn-onnx` / ONNX Runtime Web — onnx.ai.
8. Crates Rust geoespaciales/ML: `geo`, `ndarray`, `geozero`, `linfa` — docs.rs.

---

*Notas de honestidad para quien planee:* (a) la cifra de ~13 min de stats zonales y los tiempos del cuaderno provienen de corridas locales del autor sobre la escena estatal completa, no de mediciones en navegador; (b) los esfuerzos son estimaciones, el largo plazo real es el paso 3 secuencial y la arquitectura de streaming; (c) reproducir `pyshepseg` exactamente requiere igualar su orden de fusión/desempate, por lo que se valida por equivalencia de particiones (ARI), no por igualdad de IDs.
