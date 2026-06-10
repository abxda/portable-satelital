# Etiquetas urbano/no-urbano para la escena estatal de Aguascalientes (30 m).
# 1 = localidad urbana (INEGI), 2 = resto. OJO: la escena es el RECTANGULO que
# contiene al estado, asi que rasterizamos las localidades urbanas de TODOS los
# estados dentro del encuadre (si no, los pueblos vecinos entrarian al
# entrenamiento como falsos "no urbano").
import geopandas as gpd
import numpy as np
import rasterio
from rasterio import features
from shapely.geometry import box

IMG = r"D:\PortableSatelital\wasm\files\ags_landsat_30m.tif"
GPKG = r"D:\PortableSatelital\dist\data\00l.gpkg"
DST = r"D:\PortableSatelital\wasm\files\labels_ags_30m.tif"

with rasterio.open(IMG) as src:
    crs, transform, shape = src.crs, src.transform, (src.height, src.width)
    encuadre = gpd.GeoSeries([box(*src.bounds)], crs=crs)

info_crs = gpd.read_file(GPKG, rows=1).crs
bbox = tuple(encuadre.to_crs(info_crs).total_bounds)
loc = gpd.read_file(GPKG, bbox=bbox)
print("localidades en el encuadre:", len(loc),
      "| por estado:", loc.CVE_ENT.value_counts().to_dict())

urb = loc[loc.AMBITO.str.upper().str.startswith("U")].to_crs(crs)
print("urbanas:", len(urb), f"| area total: {urb.area.sum()/1e6:.0f} km2")

lab = features.rasterize(
    ((geom, 1) for geom in urb.geometry), out_shape=shape,
    transform=transform, fill=2, dtype="uint8")
print(f"pixeles urbanos: {(lab == 1).sum():,} ({(lab == 1).mean():.1%})")

perfil = dict(driver="GTiff", width=shape[1], height=shape[0], count=1,
              dtype="uint8", crs=crs, transform=transform, nodata=0,
              tiled=True, blockxsize=512, blockysize=512)
with rasterio.open(DST, "w", **perfil) as dst:
    dst.write(lab, 1)
    dst.set_band_description(1, "1=localidad urbana (INEGI), 2=resto")

# centro de la capital (componente urbana mas grande) para los acercamientos
import scipy.ndimage as ndi
cc, n = ndi.label(lab == 1)
mayor = np.argmax(np.bincount(cc.ravel())[1:]) + 1
cy, cx = ndi.center_of_mass(cc == mayor)
print(f"centro de la capital (fila, col): ({cy:.0f}, {cx:.0f})")

import os
print(f"LISTO: {DST} ({os.path.getsize(DST)/1e6:.1f} MB)")
