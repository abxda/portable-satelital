# Fix local: pyshepseg rebana objetos grandes en "tiras" (lagos, presas, manchas urbanas)

**Para:** Víctor Silva
**De:** Abel Coronado (con análisis técnico del proyecto portable-satelital)
**Fecha:** junio 2026

---

## El problema, en 30 segundos

Si segmentas con `pyshepseg` una escena que contiene un objeto homogéneo grande
(la Laguna de Yuriria, una presa, una mancha urbana compacta), el objeto sale
**rebanado en tiras paralelas onduladas** en lugar de quedar entero. RSGISLib,
con la misma imagen y parámetros equivalentes, lo entrega entero.

**No es un parámetro mal elegido y no se corrige con `minSegmentSize` ni
`maxSpectralDiff`.** La causa es una constante *hardcodeada* dentro de
`pyshepseg`:

```python
# pyshepseg/shepseg.py, dentro de clump():
MAX_CLUMP_SIZE = 10000
```

Es una optimización del port a numba que **no existe en el paper de Shepherd
ni en RSGISLib**: ningún clump puede crecer más de 10,000 píxeles (a 10 m/px
≈ 1 km²). Un lago de 80 km² queda partido en ~80 tiras — y como cada tira
supera `minSegmentSize`, la eliminación iterativa **jamás** las fusiona.

**Evidencia medida** (recorte real de Yuriria, geomediana Sentinel-2 de 12
bandas, mismo K-means en ambos casos):

| | segmento más grande | el lago |
|---|---|---|
| pyshepseg tal cual | 11,767 px | ~30 tiras |
| sin el tope | **387,989 px** | **entero** (≡ RSGISLib) |

---

## Opción 1 (recomendada): reemplazo directo por `shepherd-wasm`

Es nuestra reimplementación del mismo algoritmo en numpy/scipy puro
(sin numba), **validada bit a bit contra pyshepseg** en modo compatibilidad,
y con el tope convertido en parámetro — apagado por default.

```bash
# un solo archivo, sin instalación:
curl -L -O https://raw.githubusercontent.com/abxda/shepherd-wasm/main/shepherd_pure.py
```

```python
# en tu script, cambia:
#   from pyshepseg import shepseg
#   res = shepseg.doShepherdSegmentation(img, ...)
# por:
import shepherd_pure
res = shepherd_pure.doShepherdSegmentation(img, numClusters=60,
                                           minSegmentSize=100,
                                           imgNullVal=nodata)
# misma firma, mismo SegmentationResult. El default maxClumpSize=None
# ya entrega los objetos grandes ENTEROS.
```

Dependencias: `numpy`, `scipy`, `scikit-learn` (las tienes seguro).
Repositorio y documentación de la divergencia:
<https://github.com/abxda/shepherd-wasm>

Notas honestas: en tiles ≤512² es incluso más rápido que pyshepseg (se ahorra
el JIT); en escenas muy grandes en un solo bloque es más lento que numba —
para eso procesa por mosaico, o usa la Opción 2.

## Opción 2: parchar tu pyshepseg instalado (1 línea)

Si prefieres seguir con pyshepseg (por velocidad numba o por `tiling`),
edita la constante en tu instalación:

```bash
# 1) localiza el archivo:
python -c "import pyshepseg.shepseg as s; print(s.__file__)"

# 2) ábrelo y cambia UNA línea (está dentro de la función clump):
#       MAX_CLUMP_SIZE = 10000
#    por:
#       MAX_CLUMP_SIZE = 1000000000
```

Reinicia Python (numba captura la constante al compilar) y listo: el tope
queda efectivamente desactivado. Memoria y velocidad no sufren — la pila del
flood fill ya está dimensionada al tamaño de la imagen.

⚠️ Recuerda: el parche se pierde si reinstalas/actualizas pyshepseg.
⚠️ Si usas `pyshepseg.tiling` con escenas enormes: cada tile impone su propio
límite natural al tamaño del clump (un lago más grande que el tile se corta
en la frontera y la costura lo re-une parcialmente). Para objetos
gigantescos, valida el resultado visualmente.

## Cómo verificar que quedó corregido

```python
import numpy as np
sizes = np.bincount(res.segimg.ravel())[1:]
print("segmento más grande:", sizes.max(), "px")
# ANTES del fix: nunca verás un valor mucho mayor a ~10,000-12,000
# DESPUÉS: el lago/presa/mancha aparece con su tamaño real (cientos de miles)
```

Y el chequeo visual de siempre: las fronteras sobre el RGB — el lago debe ser
**una** región, sin rayas internas.

---

*Análisis completo y caso reproducible: repo `portable-satelital`,
`wasm/qa/caso_yuriria.py`. Cualquier duda, me dices. — Abel (abxda × Claude)*
