package main

// Modo headless (oculto, para pruebas e2e y CI — NO documentado al alumno):
//
//	SatLab.exe --headless-install   instala (catálogo firmado + sha256 + extracción)
//	SatLab.exe --headless-smoke     instala si hace falta, arranca Jupyter,
//	                                verifica HTTP 200 en /lab y lo apaga
//
// Como la app compila con -ldflags windowsgui no hay consola: el resultado
// queda en satlab-headless.log junto al exe, y en el código de salida.

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func runHeadless(mode string) int {
	a := NewApp()
	a.noBrowser = true
	f, err := os.Create(filepath.Join(root(), "satlab-headless.log"))
	if err != nil {
		return 3
	}
	defer f.Close()
	logf := func(format string, args ...any) {
		fmt.Fprintf(f, time.Now().Format("15:04:05")+" "+format+"\n", args...)
	}
	logf("modo %s · launcher v%s · root=%s", mode, AppVersion, root())

	switch mode {
	case "--headless-install":
		if err := a.doInstall(); err != nil {
			logf("INSTALL FAIL: %v", err)
			return 1
		}
		logf("INSTALL OK")
	case "--headless-smoke":
		if _, err := os.Stat(pythonExe()); err != nil {
			if err := a.doInstall(); err != nil {
				logf("INSTALL FAIL: %v", err)
				return 1
			}
			logf("INSTALL OK")
		}
		if err := a.startJupyter(); err != nil {
			logf("JUPYTER FAIL: %v", err)
			return 1
		}
		a.mu.Lock()
		url := a.labURL
		a.mu.Unlock()
		resp, err := http.Get(url)
		if err != nil {
			logf("HTTP FAIL: %v", err)
			a.StopLab()
			return 1
		}
		resp.Body.Close()
		logf("SMOKE: %s -> HTTP %d", url, resp.StatusCode)
		a.StopLab()
		if resp.StatusCode != http.StatusOK {
			return 1
		}
		logf("SMOKE OK")
	default:
		logf("modo desconocido")
		return 2
	}
	return 0
}
