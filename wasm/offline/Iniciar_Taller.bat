@echo off
rem ==========================================================================
rem  Taller Offline - doble clic y listo.
rem  Solo usa PowerShell (firmado por Microsoft, incluido en Windows).
rem  No requiere internet, ni administrador, ni instala nada.
rem ==========================================================================
title Taller Offline - Analisis de Imagenes Satelitales
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0servir_taller.ps1"
pause
