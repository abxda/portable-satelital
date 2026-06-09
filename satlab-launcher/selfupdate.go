package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/abxda/portable-satelital/satlab-launcher/internal/fetch"
	"github.com/abxda/portable-satelital/satlab-launcher/internal/sign"
)

// selfUpdateBase es la carpeta en HF donde se publican el exe del launcher y
// su descriptor de versión FIRMADO por plataforma. Override de pruebas:
// SATLAB_UPDATE_BASE.
const selfUpdateBase = "https://huggingface.co/datasets/abxda/portable-satelital/resolve/main/launchers"

func updateBase() string {
	if v := os.Getenv("SATLAB_UPDATE_BASE"); v != "" {
		return v
	}
	return selfUpdateBase
}

type latestInfo struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
}

// checkSelfUpdate corre al arrancar (goroutine): si hay versión más nueva
// publicada Y su descriptor firmado verifica, avisa a la UI. El usuario decide
// con un clic (ApplyUpdate). Silencioso ante cualquier error: el launcher
// nunca se bloquea por el update.
func (a *App) checkSelfUpdate() {
	// limpia un .old de una actualización previa (best-effort)
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}
	info, err := fetchLatestSigned()
	if err != nil || info.Version == "" {
		return
	}
	if !versionNewer(info.Version, AppVersion) {
		return
	}
	a.emit(uiEvent{Kind: "update", Msg: info.Version})
}

// ApplyUpdate descarga el exe nuevo (verificado por SHA-256 del descriptor
// FIRMADO), se reemplaza a sí mismo y relanza. Patrón del Big Data lab:
// actual -> .old, nuevo -> actual (en Windows se puede renombrar un exe en
// ejecución, no borrarlo).
func (a *App) ApplyUpdate() {
	go func() {
		info, err := fetchLatestSigned()
		if err != nil {
			a.fail("No pude verificar la actualización: " + err.Error())
			return
		}
		exe, err := os.Executable()
		if err != nil {
			a.fail(err.Error())
			return
		}
		a.emit(uiEvent{Kind: "progress", Phase: "Descargando la actualización…", Pct: -1})
		newPath := exe + ".new"
		entry := fetch.Entry{Base: "selfupdate", File: filepath.Base(info.URL), URL: info.URL, SHA256: info.SHA256}
		if err := fetch.Download(entry, newPath, nil); err != nil {
			os.Remove(newPath)
			a.fail("Descarga de la actualización falló: " + err.Error())
			return
		}
		oldPath := exe + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(exe, oldPath); err != nil {
			os.Remove(newPath)
			a.fail("No pude preparar el reemplazo: " + err.Error())
			return
		}
		if err := os.Rename(newPath, exe); err != nil {
			_ = os.Rename(oldPath, exe) // revierte
			a.fail("No pude colocar la versión nueva: " + err.Error())
			return
		}
		a.log("ok", "Actualizado a la versión "+info.Version+". Reiniciando…")
		cmd := exec.Command(exe)
		cmd.Dir = filepath.Dir(exe)
		_ = cmd.Start()
		time.Sleep(400 * time.Millisecond)
		wruntime.Quit(a.ctx)
	}()
}

// fetchLatestSigned baja el descriptor del launcher Y su firma .sig, verifica
// la firma Ed25519 con la llave embebida y solo entonces lo parsea. Así ni el
// mecanismo de actualización confía en el hosting.
func fetchLatestSigned() (latestInfo, error) {
	var info latestInfo
	base := fmt.Sprintf("%s/satlab-latest-%s-%s.json", updateBase(), runtime.GOOS, runtime.GOARCH)
	raw, err := fetch.FetchBytes(base)
	if err != nil {
		return info, err
	}
	sig, err := fetch.FetchBytes(base + ".sig")
	if err != nil {
		return info, fmt.Errorf("descriptor sin firma: %w", err)
	}
	if err := sign.Verify(raw, sig); err != nil {
		return info, err
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return info, err
	}
	if info.SHA256 == "" || info.URL == "" {
		return info, fmt.Errorf("descriptor incompleto")
	}
	return info, nil
}

// versionNewer compara "X.Y.Z" numéricamente, parte por parte.
func versionNewer(remote, local string) bool {
	rp := strings.Split(strings.TrimPrefix(remote, "v"), ".")
	lp := strings.Split(strings.TrimPrefix(local, "v"), ".")
	for i := 0; i < len(rp) || i < len(lp); i++ {
		r, l := 0, 0
		if i < len(rp) {
			r, _ = strconv.Atoi(rp[i])
		}
		if i < len(lp) {
			l, _ = strconv.Atoi(lp[i])
		}
		if r != l {
			return r > l
		}
	}
	return false
}
