# QA: abre SatLab, clic real en "Abrir laboratorio", espera Jupyter, cierra la
# ventana con la X (WM_CLOSE) y verifica que NO queden procesos ni puerto.
# Uso: qa_close_window.ps1 <carpeta-del-lab>
param([string]$LabRoot = "D:\SatLab-PruebaAlumno")
$ErrorActionPreference = "Stop"
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WinQA {
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint dx, uint dy, uint d, UIntPtr e);
  [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr h, uint m, IntPtr w, IntPtr l);
  public struct RECT { public int Left, Top, Right, Bottom; }
}
"@
$p = Start-Process -FilePath (Join-Path $LabRoot "SatLab.exe") -PassThru
Start-Sleep -Seconds 7
$h = (Get-Process -Id $p.Id).MainWindowHandle
[WinQA]::SetForegroundWindow($h) | Out-Null
Start-Sleep -Milliseconds 300
$r = New-Object WinQA+RECT
[WinQA]::GetWindowRect($h, [ref]$r) | Out-Null
[WinQA]::SetCursorPos($r.Left + 450, $r.Top + 343) | Out-Null
Start-Sleep -Milliseconds 200
[WinQA]::mouse_event(2, 0, 0, 0, [UIntPtr]::Zero)
[WinQA]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)
Write-Output "clic en Abrir laboratorio enviado"
Start-Sleep -Seconds 18
$py = @(Get-Process python -ErrorAction SilentlyContinue | Where-Object { $_.Path -and $_.Path.StartsWith($LabRoot) })
$port = Get-NetTCPConnection -LocalPort 8888 -State Listen -ErrorAction SilentlyContinue
Write-Output ("ANTES de cerrar -> python del lab: {0}  puerto 8888 escuchando: {1}" -f $py.Count, [bool]$port)
Write-Output "--- cerrando la ventana con la X (WM_CLOSE) ---"
[WinQA]::PostMessage($h, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero) | Out-Null
Start-Sleep -Seconds 6
$alive = Get-Process -Id $p.Id -ErrorAction SilentlyContinue
$py2 = @(Get-Process python -ErrorAction SilentlyContinue | Where-Object { $_.Path -and $_.Path.StartsWith($LabRoot) })
$port2 = Get-NetTCPConnection -LocalPort 8888 -State Listen -ErrorAction SilentlyContinue
Write-Output ("DESPUES de la X -> SatLab vivo: {0}  python del lab: {1}  puerto 8888 escuchando: {2}" -f [bool]$alive, $py2.Count, [bool]$port2)
if (-not $alive -and $py2.Count -eq 0 -and -not $port2) { Write-Output "CIERRE LIMPIO OK" } else { Write-Output "RESIDUOS DETECTADOS"; exit 1 }
