# Curso "Análisis de Imágenes Satelitales con Machine Learning" — descripción de láminas

Documento de referencia de la presentación teórica (26 láminas), derivada de la
página interactiva `teoria.html`. Marca institucional SNIEG (filete de 4 colores
+ logo). Cada lámina se describe de forma objetiva; el orden es el del pase de
diapositivas. Las láminas "figura" son SVG animados e interactivos en la web; en
el PDF se capturan como imagen estática (24 de las 26 láminas; las dos celdas de
texto largas y la tabla extendida de la figura 6 se omiten o resumen en el PDF).

---

## Lámina 1 — Portada
Título del curso "Machine Learning para detectar zonas urbanas", subtítulo (percepción remota, geomediana, objetos, aprendizaje automático y las dos plataformas) y autoría: **Abel Alejandro Coronado Iruegas · INEGI — Dirección General de Integración, Análisis e Investigación · Dirección General Adjunta de Investigación · Laboratorio de Ciencia de Datos**.

## Lámina 2 — Teoría 1: ¿Cómo "ve" un satélite? (texto)
Define **percepción remota**: medir desde el espacio la luz solar reflejada por la superficie, sin contacto. Presenta **Sentinel-2** (Copernicus): cobertura global cada ~5 días, píxeles de 10×10 m, datos públicos y gratuitos.

## Lámina 3 — Figura 1: el sensor pasivo (animada)
Cadena de energía: el **Sol** (esquina izquierda, con rayos radiales) ilumina; **fotones dorados** bajan y **rebotan** en el terreno cambiando a colores de banda; la mayoría escapa al espacio y el satélite captura una muestra. Terreno isométrico con cultivos (surcos), agua y **edificios con ventanas**. Panel "lecturas del sensor" con 6 barras espectrales sincronizadas al barrido (campo: NIR alto; ciudad: rojo y SWIR altos). Mensaje: el satélite no emite luz ni toma "fotos", **sensa la intensidad del reflejo por banda**.

## Lámina 4 — Teoría 2: el píxel es una firma espectral (texto)
Un píxel no es un color: es un **vector de mediciones** (firma espectral). Sentinel-2 registra 12 bandas; el taller usa la versión "calidad Landsat" (6 bandas, 30 m). La firma es la idea central: el modelo verá firmas, no colores.

## Lámina 5 — Figura 2: firmas espectrales (interactiva)
Cuatro parches seleccionables — **💧 agua, 🌿 vegetación, 🏙️ ciudad, 🟤 suelo seco** — con su firma de 12 bandas en barras; se marcan las 6 bandas "calidad Landsat" del taller y las zonas del espectro (visible / borde rojo+NIR / SWIR). Punto clave: suelo y ciudad tienen firmas **casi gemelas** → justifica usar todas las bandas y el enfoque por objetos.

## Lámina 6 — Teoría 2b: la imagen es un cubo (texto)
La imagen no es plana: cada píxel guarda un número por banda (**profundidad espectral**) y cada fecha es otra toma (**resolución temporal**). El resultado es un **cubo de datos** (ancho × alto × bandas × tiempo); algunas fechas traen nubes, que la geomediana descartará.

## Lámina 7 — Figura 2b: el cubo de datos (animada, un renglón)
Leyenda de las 6 bandas; **cubo isométrico** de una toma con el valor 0–1 en cada píxel y **píxeles de nube** explícitos (blancos). Flecha "× el tiempo" → **recta de fechas** (12 ene · 13 feb · 18 mar) con un mini-cubo multiespectral sobre cada fecha (uno "con nube"), apareciendo escalonados. Cierre: la resolución temporal apila tomas → la serie que la geomediana resume.

## Lámina 8 — Teoría 3: la geomediana (texto)
Pipeline en 4 pasos (metodología INEGI / Cubo de Datos Geoespaciales): (1) pila de todas las pasadas del periodo; (2) máscara de calidad que descarta nube/sombra/saturación; (3) **geomediana** por píxel (mediana geométrica, Roberts et al. 2017) calculada con el algoritmo de **Weiszfeld**, preservando las relaciones entre bandas; (4) mosaico. Datos del taller: `GM_AGS_2020.tif` (Zenodo) + referencia al documento metodológico de INEGI.

## Lámina 9 — Figura 3: pipeline de la geomediana (animada)
Cuatro etapas: **1·** la pila del periodo (cubos colapsando); **2·** máscara de calidad (nubes con ✕ rojas que desaparecen; robusta hasta ~50 % de ruido); **3·** un píxel de cerca → la **geomediana** en el espacio espectral con el **caminante de Weiszfeld** animado (parte del promedio, afectado por atípicos, hasta converger); **4·** mosaico limpio con **teselas de 3×3 píxeles** (grilla 9×6) y ventajas (sin nubes, preserva relaciones entre bandas → NDVI/NDBI/ML). Pie con datos del CDGM: +118 mil escenas Landsat desde 1984, 6 bandas a 30 m, ~24 h, 35 GB.

## Lámina 10 — Figura 3b: navegador de geomedianas INEGI (interactiva, mapa en vivo)
Visor Leaflet que consume **en vivo** el WMS público del INEGI con la serie nacional de geomedianas Landsat **1984–2024**. Controles: selector de período, tres visualizaciones (**🌈 color natural · 🌿 falso color · 🔥 infrarrojo**) y **🆚 comparar años** con cortina deslizable (mapa de fondo fijo). Centrado en Aguascalientes. Cita y liga a inegi.org.mx/programas/geomediana.

## Lámina 11 — Teoría 4: del píxel al objeto (texto)
La clasificación píxel a píxel produce "sal y pimienta"; el enfoque **GEOBIA** agrupa primero en objetos. Segmentación de **Shepherd, Bunting & Dymond (2019)** en 3 subprocesos: siembra K-means → aglomerado (clumping) → eliminación iterativa. Parámetros clave: `numClusters` y `minSegmentSize`.

## Lámina 12 — Figura 4: mini-segmentador Shepherd (interactiva)
El algoritmo **real** corriendo en la página sobre un paisaje en miniatura (cultivos, una presita de 4 px, ciudad y un parque de 4 px). Controles: `numClusters` (3/5/8), `minSegmentSize` (1/4/10 px), botón "▶ ver el proceso" y pestañas por subproceso (**① familias · ② grupitos · ③ objetos**). Contadores en vivo (familias → grupitos → objetos) y diagnóstico honesto de qué subproceso "mató" un objeto chico.

## Lámina 13 — Teoría 5: la verdad-terreno (texto)
El aprendizaje supervisado necesita ejemplos etiquetados confiables: el **Marco Geoestadístico del Censo 2020 (INEGI)**, polígonos de localidades urbanas y rurales amanzanadas. Ambas forman la clase URBANA (1); el resto, NO URBANA (2). El dato vectorial y la imagen ráster hablan idiomas distintos → hay que traducir (rasterizar).

## Lámina 14 — Figura 5: vector → píxel (interactiva)
Izquierda: el **polígono vectorial** de INEGI (vértices visibles + ficha de atributos NOMGEO/AMBITO). Botón "▶ rasterizar" anima la traducción con barrido y **point-in-polygon real**: cada celda pregunta si su centro cae dentro → 1 o 2. Botón "👁 ver alineación" encima la máscara sobre la imagen (misma malla). Conteo real de píxeles por clase.

## Lámina 15 — Teoría 6: de la imagen a la tabla (texto)
Los algoritmos comen **tablas**: por cada objeto, estadísticas zonales (media y desviación por banda) + proporción urbana → una fila por objeto, una columna por característica. Antes con la Raster Attribute Table de RSGISLib; hoy con `numpy`.

## Lámina 16 — Figura 6: estadística zonal y agregación (interactiva)
Se elige un objeto (**🌾 parcela · 💧 presita · 🏙️ manzana**) y se anima: objeto ∩ imagen → recorte (píxeles vuelan) → **6 pilas de bandas** → paso 4: **agregación**. Una banda (NIR) se muestra como **pila de números 0–1**; una llave + **Σ** la resumen en un valor; caja con suma, **media μ**, varianza σ², mín y máx; flecha "× 6 bandas (μ·σ²·mín·máx)" → la fila como **DataFrame** (`#id · n_px · b1μ…b6μ`). Debajo, la **tabla completa** con los 3 objetos y más métricas (μ y σ de cada banda). *(En el PDF se muestra el gráfico; la tabla extendida sólo en la web.)*

## Lámina 17 — Teoría 7: aprendizaje automático (texto)
Receta del taller: depurar (objetos puros ≥90 %), balancear clases, separar 70/30, entrenar (StandardScaler → MLP apilado → ExtraTrees) y evaluar con matriz de confusión sobre el 30 % no visto. Exactitud de referencia ~94 %.

## Lámina 18 — Figura 7: pipeline de clasificación apilado (animada)
Botón "▶ Entrenar". Seis pasos: dataset 70/30 → **StandardScaler** (escala pareja) → la **red MLP opina** (P(urb)/P(no)) → **stacking**: la opinión se **anexa** al vector (12 → 15 características) → **ExtraTrees** vota (conteo en vivo, p. ej. 87/13 → clase URBANO) → **examen final** (matriz de confusión + medidor a ~94 %).

## Lámina 19 — Teoría 8: ¿qué es un cuaderno Jupyter? (texto)
El cuaderno como libreta de laboratorio: celdas de texto y celdas de **código** ejecutables con Shift+Enter que muestran su resultado debajo. Ideal para aprender: leer, ejecutar, ver el efecto, cambiar y repetir.

## Lámina 20 — Figura 8: cuaderno en acción (interactiva)
Maqueta de un `.ipynb` con celdas; botón "▶ Ejecutar el cuaderno" que muestra los contadores de ejecución `[*] → [1]/[2]` y las salidas apareciendo bajo cada celda (texto y un mini-resultado gráfico).

## Lámina 21 — Teoría 9: ¿dónde corre Python? WebAssembly (texto)
WASM + Pyodide permiten correr Python científico **dentro del navegador**, en una caja de arena: cero instalación, cero permisos, los datos no salen del equipo, sin rastro al cerrar. Plantea la anatomía del sandbox y la pregunta "¿qué pasa si borro todos los archivos?".

## Lámina 22 — Figura 9: anatomía del sandbox (interactiva)
Tres "casas" de los archivos y 5 escenarios con botón: **▶ abres el taller** (el servidor de solo lectura entrega una copia), **✏️ guardas Ctrl+S** (va al almacén del navegador / IndexedDB), **❌ cierras la pestaña** (el sandbox y /tmp se esfuman; el almacén sobrevive), **🗑️ ¡borras TODO!** (solo borras tus copias; al recargar se restauran los originales), **🔓 ¿y mi disco C:\?** (el sandbox no puede verlo: caja de arena de verdad).

## Lámina 23 — Teoría 10: el laboratorio portable (texto)
**SatLab** para cuando los datos crecen (Sentinel-2 completa, 10 m, 12 bandas, 2.66 GB). Tabla comparativa navegador vs portable. **Transparencia**: el portable instala un Python real → poder (GB, núcleos, Google Earth Engine) y también exposición; en equipo institucional, **solicitar autorización del área de TI** y revisar la estrategia interna; se ofrece la cadena de verificación (VERIFICACION.md).

## Lámina 24 — Figura 10: arquitectura del portable (animada)
Flujo de instalación encendiéndose por pasos: `SatLab.exe` (un archivo, sin admin) → descarga con **cadena de verificación** (SHA-256 · firma Ed25519 · atestación pública del CI) → **una carpeta** (python + JupyterLab + stack) → corre en `127.0.0.1` → desinstalar = borrar la carpeta. Contraste honesto en positivo: 🌐 navegador (caja sellada, exposición nula) vs 💻 portable (Python real: poder + exposición). Banda ámbar: autorización de TI + VERIFICACION.md.

## Lámina 25 — Cierre: el mapa completo del taller (texto)
Tabla-resumen de los 6 pasos del taller (imagen limpia anual, del píxel al objeto, verdad-terreno, imagen→tabla, entrenar/evaluar, mapa + descargas) con su herramienta (geomediana, `shepherd-wasm`, Marco Geoestadístico, numpy, scikit-learn, geopandas). Botones para abrir el taller y volver a la portada. Pie con referencias.

## Lámina 26 — Figura 11: el viaje completo (animada)
Mapa-resumen del método en **8 estaciones** sobre una vía: ☀️ luz y firmas → 🧪 geomediana → 🛰️ calidad Landsat 30 m → 🧩 segmentación Shepherd → 🏷️ etiquetas INEGI→píxel → 📋 tabla de objetos → 🤖 stacking MLP+árboles → 🗺️ mapa urbano estatal. Un "tren" recorre la vía y cada estación pulsa a su paso. Remate: "en el navegador hoy · en el portable cuando tu institución lo autorice · con TUS datos cuando quieras".

---

*Funciones de presentación de la página: cada lámina se maximiza (⛶) a pantalla completa con el encabezado SNIEG; navegación anterior/siguiente (botones ‹ › o flechas del teclado / control remoto); pausa global de animaciones (tecla P). PDF descargable: `Curso_Teoria_SNIEG.pdf`.*
