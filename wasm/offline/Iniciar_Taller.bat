@echo off
rem ==========================================================================
rem  Taller Offline - doble clic y listo. NO usa PowerShell.
rem  Servidor: python.exe OFICIAL de python.org, incluido en la carpeta
rem  python\ y FIRMADO digitalmente por la Python Software Foundation
rem  (clic derecho a python\python.exe - Propiedades - Firmas digitales).
rem  Sirve SOLO en 127.0.0.1 (este equipo), solo lectura, sin administrador.
rem  Para detener: cerrar esta ventana.
rem ==========================================================================
title Taller Offline - Analisis de Imagenes Satelitales
cd /d "%~dp0"
echo.
echo   Taller Offline sirviendo SOLO en este equipo:
echo   http://127.0.0.1:8765/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb
echo.
echo   (tu navegador se abrira en unos segundos; cierra esta ventana para detener)
echo.
start "" cmd /c "timeout /t 2 >nul & start http://127.0.0.1:8765/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb"
python\python.exe -m http.server 8765 --bind 127.0.0.1 -d sitio
pause
