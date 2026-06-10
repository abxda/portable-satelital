# Reproduce el artefacto de "tiras" de pyshepseg en el recorte del Dr.
# (Laguna de Yuriria) y demuestra el fix maxClumpSize=None de shepherd_pure.
import sys
import time

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import rasterio
from scipy import ndimage

sys.path.insert(0, r"D:\PortableSatelital\wasm")
import shepherd_pure
from pyshepseg import shepseg

SRC = r"D:\PortableSatelital\caso\crop_qgis_user.tif"
with rasterio.open(SRC) as s:
    img = s.read().astype(np.float32)
print("imagen:", img.shape)

# KMeans UNICO compartido: la comparacion aisla el clump/eliminacion
km = shepseg.fitSpectralClusters(img, 60, 1, None, True)

t0 = time.time()
ref = shepseg.doShepherdSegmentation(img, kmeansObj=km, minSegmentSize=100,
                                     imgNullVal=None)
print(f"pyshepseg (tope 10k): {int(ref.segimg.max()):,} segmentos en {time.time()-t0:.0f}s")

t0 = time.time()
fix = shepherd_pure.doShepherdSegmentation(img, kmeansObj=km, minSegmentSize=100,
                                           imgNullVal=None, maxClumpSize=None)
print(f"shepherd_pure SIN tope: {int(fix.segimg.max()):,} segmentos en {time.time()-t0:.0f}s")

# tamaño del segmento mayor (el lago) en cada caso
for nombre, seg in (("pyshepseg", ref.segimg), ("sin tope", fix.segimg)):
    sizes = np.bincount(seg.ravel())[1:]
    print(f"  {nombre}: segmento mayor = {sizes.max():,} px "
          f"({sizes.max()/seg.size:.1%} de la imagen)")

# render comparativo (RGB 4-3-2 + fronteras)
rgb = np.stack([img[3], img[2], img[1]], axis=-1)
p2, p98 = np.percentile(rgb, (2, 98))
rgb = np.clip((rgb - p2) / (p98 - p2), 0, 1)

fig, axes = plt.subplots(1, 2, figsize=(17, 5.2))
for ax, (titulo, seg) in zip(axes, [
        (f"pyshepseg (MAX_CLUMP_SIZE=10000) — {int(ref.segimg.max()):,} seg.", ref.segimg),
        (f"shepherd-wasm maxClumpSize=None — {int(fix.segimg.max()):,} seg.", fix.segimg)]):
    b = ndimage.maximum_filter(seg, 2) != ndimage.minimum_filter(seg, 2)
    v = rgb.copy()
    v[b] = [1, 1, 0]
    ax.imshow(v)
    ax.set_title(titulo, fontsize=11)
    ax.axis("off")
plt.tight_layout()
plt.savefig(r"D:\PortableSatelital\dist\caso_comparacion.png", dpi=110, bbox_inches="tight")
print("comparacion guardada")
