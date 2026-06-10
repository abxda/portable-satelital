# Exporta las localidades del Marco Geoestadistico (INEGI) que caen en el
# encuadre estatal, como GeoJSON WGS84 (RFC 7946). El CUADERNO del taller hace
# el resto del linaje en vivo: filtrar urbanas -> proyectar -> rasterizar.
import geopandas as gpd
import rasterio
from shapely.geometry import box

IMG = r"D:\PortableSatelital\wasm\files\ags_landsat_30m.tif"
GPKG = r"D:\PortableSatelital\dist\data\00l.gpkg"
DST = r"D:\PortableSatelital\wasm\files\localidades_encuadre.geojson"

with rasterio.open(IMG) as src:
    encuadre = gpd.GeoSeries([box(*src.bounds)], crs=src.crs)

crs_gpkg = gpd.read_file(GPKG, rows=1).crs
loc = gpd.read_file(GPKG, bbox=tuple(encuadre.to_crs(crs_gpkg).total_bounds))
loc = loc[["CVEGEO", "NOMGEO", "AMBITO", "CVE_ENT", "geometry"]].to_crs("EPSG:4326")
print("localidades:", len(loc), "| urbanas:", (loc.AMBITO == "Urbana").sum())

loc.to_file(DST, driver="GeoJSON")
import os
print(f"LISTO: {DST} ({os.path.getsize(DST)/1e6:.1f} MB)")
