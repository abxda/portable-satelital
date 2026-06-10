# validate_shepherd.py — valida shepherd_pure.py contra pyshepseg (oráculo).
#
# Diseño de la validación:
#   * recorte real de la geomediana (512x512x12 por defecto) — no sintético;
#   * UN solo KMeans compartido (kmeansObj) para ambas implementaciones:
#     así la comparación aísla clump + eliminación (lo que se reimplementó)
#     de la aleatoriedad del paso 1;
#   * img en float32 para ambas (evita wraparound de enteros sin signo en las
#     distancias de la referencia numba);
#   * métricas: ARI sobre píxeles no nulos, nº de segmentos, conteos de
#     eliminados, y cumplimiento de minSegmentSize.
#
# Uso (con el python del portable, que trae pyshepseg+numba):
#   python validate_shepherd.py <imagen.tif> [row_off col_off size minSegSize]

import sys
import time

import numpy as np
import rasterio
from rasterio.windows import Window
from sklearn.metrics import adjusted_rand_score

from pyshepseg import shepseg
import shepherd_pure


def main():
    tif = sys.argv[1]
    row_off = int(sys.argv[2]) if len(sys.argv) > 2 else 5000
    col_off = int(sys.argv[3]) if len(sys.argv) > 3 else 5500
    size = int(sys.argv[4]) if len(sys.argv) > 4 else 512
    minSeg = int(sys.argv[5]) if len(sys.argv) > 5 else 50

    with rasterio.open(tif) as src:
        img = src.read(window=Window(col_off, row_off, size, size))
        nodata = src.nodata
        print(f"ventana {size}x{size} en ({row_off},{col_off})  bandas={img.shape[0]} "
              f"dtype={img.dtype} nodata={nodata}")

    img = img.astype(np.float32)
    if nodata is not None:
        nullFrac = float((img == nodata).any(axis=0).mean())
        print(f"fraccion nula: {nullFrac:.1%}")
        if nullFrac > 0.5:
            print("ADVERTENCIA: mas de la mitad nula; elige otra ventana")

    # KMeans UNICO y determinista, compartido por ambas implementaciones
    km = shepseg.fitSpectralClusters(img, 60, 1, nodata, True)

    t0 = time.perf_counter()
    ref = shepseg.doShepherdSegmentation(
        img, kmeansObj=km, minSegmentSize=minSeg, imgNullVal=nodata)
    tRef = time.perf_counter() - t0
    print(f"\npyshepseg : {int(ref.segimg.max())} segmentos, "
          f"singles={ref.singlePixelsEliminated} small={ref.smallSegmentsEliminated}, "
          f"maxSpectralDiff={ref.maxSpectralDiff:.4f}, {tRef:.1f}s")

    t0 = time.perf_counter()
    # maxClumpSize=10000: modo COMPATIBILIDAD para validar bit a bit contra
    # pyshepseg (que trae ese tope con su artefacto de tiras). El default de
    # shepherd_pure es None = fiel al algoritmo/RSGISLib.
    mine = shepherd_pure.doShepherdSegmentation(
        img, kmeansObj=km, minSegmentSize=minSeg, imgNullVal=nodata,
        maxClumpSize=10000)
    tMine = time.perf_counter() - t0
    print(f"puro      : {int(mine.segimg.max())} segmentos, "
          f"singles={mine.singlePixelsEliminated} small={mine.smallSegmentsEliminated}, "
          f"maxSpectralDiff={mine.maxSpectralDiff:.4f}, {tMine:.1f}s")

    a = ref.segimg.ravel()
    b = mine.segimg.ravel()
    nonnull = (a != 0) | (b != 0)
    ari = adjusted_rand_score(a[nonnull], b[nonnull])
    identical = float((a == b).mean())

    # minSegmentSize: ningún segmento < minSeg salvo los "atascados" (la
    # referencia también los deja; comparamos el CONTEO de ambos)
    def smallCount(seg):
        s = np.bincount(seg.ravel())[1:]
        s = s[s > 0]
        return int((s < minSeg).sum())

    print(f"\nARI                 : {ari:.6f}")
    print(f"pixeles identicos   : {identical:.2%} (IDs crudos; ARI es la metrica)")
    print(f"segs < minSegmentSize: ref={smallCount(ref.segimg)}  puro={smallCount(mine.segimg)}")
    print(f"velocidad           : puro/numba = {tMine / max(tRef, 1e-9):.1f}x")

    ok = ari >= 0.99
    print("\nVEREDICTO:", "PASA (ARI >= 0.99)" if ok else "FALLA (ARI < 0.99)")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
