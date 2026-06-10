# Simula una imagen "calidad Landsat" desde la geomediana Sentinel-2:
# 6 bandas (azul, verde, rojo, NIR, SWIR1, SWIR2) y 30 m de resolucion.
# Todo el estado de Aguascalientes en ~150 MB -> apto para wasm32.
import numpy as np
import rasterio
from rasterio.warp import reproject, Resampling
from rasterio.transform import Affine

SRC = r"D:\PortableSatelital\imagen\GM_AGS_2020.tif"
DST = r"D:\PortableSatelital\wasm\files\ags_landsat_30m.tif"
# orden en la geomediana: B01 B02 B03 B04 B05 B06 B07 B08 B8A B09 B11 B12
BANDAS = [2, 3, 4, 8, 11, 12]          # 1-indexadas: B02,B03,B04,B08,B11,B12
NOMBRES = ["azul (B02)", "verde (B03)", "rojo (B04)",
           "NIR (B08)", "SWIR1 (B11)", "SWIR2 (B12)"]
FACTOR = 3                              # 10 m -> 30 m

with rasterio.open(SRC) as src:
    dst_h = int(np.ceil(src.height / FACTOR))
    dst_w = int(np.ceil(src.width / FACTOR))
    dst_transform = src.transform * Affine.scale(FACTOR)
    perfil = dict(driver="GTiff", width=dst_w, height=dst_h, count=len(BANDAS),
                  dtype="int16", crs=src.crs, transform=dst_transform,
                  nodata=-9999, tiled=True, blockxsize=512, blockysize=512)
    with rasterio.open(DST, "w", **perfil) as dst:
        for i, b in enumerate(BANDAS, start=1):
            datos = src.read(b)
            sal = np.empty((dst_h, dst_w), dtype="int16")
            # reproject con average EXCLUYE nodata del promedio (a diferencia
            # del decimated read, que lo mezcla)
            reproject(datos, sal,
                      src_transform=src.transform, src_crs=src.crs,
                      dst_transform=dst_transform, dst_crs=src.crs,
                      src_nodata=-9999, dst_nodata=-9999,
                      resampling=Resampling.average)
            dst.write(sal, i)
            dst.set_band_description(i, NOMBRES[i - 1])
            print(f"banda {i} ({NOMBRES[i-1]}): "
                  f"validos {(sal != -9999).mean():.1%}", flush=True)

import os
print(f"\nLISTO: {DST}")
print(f"{dst_w} x {dst_h} px ({dst_w*dst_h/1e6:.1f} Mpx), 6 bandas int16")
print(f"tamano en disco: {os.path.getsize(DST)/1e6:.0f} MB")
