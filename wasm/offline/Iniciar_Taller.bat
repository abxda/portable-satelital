@echo off
rem ==========================================================================
rem  Taller Offline - doble clic y listo. SIN PowerShell, SIN binarios
rem  incluidos: usa IIS Express, el servidor ligero OFICIAL de Microsoft.
rem
rem  REQUISITO (lo instala TI una sola vez, con el MSI oficial de Microsoft):
rem    https://www.microsoft.com/en-us/download/details.aspx?id=48264
rem
rem  Seguridad: binario firmado por Microsoft Corporation, corre como
rem  usuario normal, sirve SOLO en localhost, solo lectura de sitio\.
rem  Para detener: cerrar esta ventana.
rem ==========================================================================
title Taller Offline - Analisis de Imagenes Satelitales
cd /d "%~dp0"

set "IISX=%ProgramFiles%\IIS Express\iisexpress.exe"
if not exist "%IISX%" set "IISX=%ProgramFiles(x86)%\IIS Express\iisexpress.exe"
if not exist "%IISX%" goto :falta

echo.
echo   Taller Offline sirviendo SOLO en este equipo:
echo   http://localhost:8765/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb
echo.
echo   (tu navegador se abrira en unos segundos; cierra esta ventana para detener)
echo.
start "" cmd /c "timeout /t 3 >nul & start http://localhost:8765/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb"
"%IISX%" /path:"%~dp0sitio" /port:8765
goto :fin

:falta
echo.
echo   Falta IIS Express (el servidor oficial gratuito de Microsoft).
echo   Pide a tu area de TI instalarlo desde:
echo   https://www.microsoft.com/en-us/download/details.aspx?id=48264
echo   (es un MSI firmado por Microsoft; se instala una sola vez)
echo.

:fin
pause
