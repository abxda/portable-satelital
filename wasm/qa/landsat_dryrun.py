# Ensayo del ejercicio "todo Aguascalientes calidad Landsat" en CPython.
# Mide tiempo por fase y pico de memoria (PeakWorkingSetSize del proceso):
# si el pico queda bajo ~2.5 GB, cabe en wasm32 con margen.
import ctypes, ctypes.wintypes as wt
import time

import numpy as np
import rasterio
import shepherd_wasm


def pico_mb():
    class PMC(ctypes.Structure):
        _fields_ = [("cb", wt.DWORD), ("PageFaultCount", wt.DWORD)] + [
            (n, ctypes.c_size_t) for n in
            ("PeakWorkingSetSize", "WorkingSetSize", "QuotaPeakPagedPoolUsage",
             "QuotaPagedPoolUsage", "QuotaPeakNonPagedPoolUsage",
             "QuotaNonPagedPoolUsage", "PagefileUsage", "PeakPagefileUsage")]
    pmc = PMC(cb=ctypes.sizeof(PMC))
    ctypes.windll.psapi.GetProcessMemoryInfo(
        ctypes.windll.kernel32.GetCurrentProcess(), ctypes.byref(pmc), pmc.cb)
    return pmc.PeakWorkingSetSize / 1e6


t0 = time.time()
with rasterio.open(r"D:\PortableSatelital\wasm\files\ags_landsat_30m.tif") as src:
    img = src.read().astype(np.float32)
print(f"[{time.time()-t0:6.1f}s] carga: {img.shape}, pico {pico_mb():.0f} MB", flush=True)

t1 = time.time()
res = shepherd_wasm.doShepherdSegmentation(
    img, numClusters=60, minSegmentSize=50, maxSpectralDiff="auto",
    imgNullVal=-9999, fixedKMeansInit=True)
seg = res.segimg
nseg = int(seg.max())
print(f"[{time.time()-t1:6.1f}s] segmentacion: {nseg:,} segmentos, "
      f"pico {pico_mb():.0f} MB", flush=True)

t2 = time.time()
sizes = np.bincount(seg.ravel())[1:]
medias = np.empty((nseg, img.shape[0]), dtype=np.float32)
flat = seg.ravel()
for b in range(img.shape[0]):
    medias[:, b] = (np.bincount(flat, weights=img[b].ravel())[1:] /
                    np.maximum(sizes, 1))
print(f"[{time.time()-t2:6.1f}s] caracteristicas {medias.shape}, "
      f"pico {pico_mb():.0f} MB", flush=True)

print(f"\nsegmento mas grande: {sizes.max():,} px "
      f"({sizes.max()*900/1e6:.1f} km2 a 30 m)")
print(f"TOTAL: {time.time()-t0:.0f} s | PICO DE MEMORIA: {pico_mb():.0f} MB")
print("VEREDICTO CPython: " + ("CABE en wasm32 (<2500 MB)"
      if pico_mb() < 2500 else "NO cabe en wasm32"))
