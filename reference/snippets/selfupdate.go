package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// selfUpdateBase es la carpeta en Hugging Face donde se publican el exe del
// launcher y su descriptor de versión por plataforma.
const selfUpdateBase = "https://huggingface.co/datasets/abxda/bdp-lab/resolve/main/launchers"

type latestInfo struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
}

// checkSelfUpdate consulta si hay un launcher MÁS NUEVO publicado para esta
// plataforma. De haberlo, descarga el exe (pequeño, ~12 MB), verifica su
// SHA-256, se reemplaza a sí mismo y RELANZA la versión nueva. Devuelve true si
// aplicó una actualización (en ese caso el proceso actual ya está saliendo y el
// startup NO debe seguir arrancando servicios).
//
// Idea clave: el stack pesado (~2.5 GB) NO se vuelve a bajar nunca; solo el exe.
// Por eso una mejora del launcher cuesta 12 MB y no 2.5 GB.
func (a *App) checkSelfUpdate() bool {
	// limpia un .old que haya quedado de una actualización previa (best-effort).
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}

	info, err := fetchLatest()
	if err != nil {
		a.setupSink.Emit("INFO", "[update] sin info de versión (continúo): "+err.Error())
		return false
	}
	if info.Version == "" || info.Version == AppVersion {
		a.setupSink.Emit("INFO", "[update] estás en la última versión ("+AppVersion+")")
		return false
	}
	a.setupSink.Emit("INFO", fmt.Sprintf("[update] hay una versión nueva del launcher: %s (tienes %s). Descargando (~12 MB)…", info.Version, AppVersion))

	exe, err := os.Executable()
	if err != nil {
		return false
	}
	newPath := exe + ".new"
	if err := downloadVerified(info.URL, newPath, info.SHA256); err != nil {
		a.setupSink.Emit("WARN", "[update] descarga falló (sigo con la versión actual): "+err.Error())
		_ = os.Remove(newPath)
		return false
	}
	// En Windows se puede renombrar el .exe en ejecución (no borrarlo). Hacemos
	// swap: actual -> .old, nuevo -> actual.
	oldPath := exe + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(exe, oldPath); err != nil {
		a.setupSink.Emit("WARN", "[update] no pude mover el exe actual: "+err.Error())
		_ = os.Remove(newPath)
		return false
	}
	if err := os.Rename(newPath, exe); err != nil {
		a.setupSink.Emit("WARN", "[update] no pude colocar el exe nuevo, revierto: "+err.Error())
		_ = os.Rename(oldPath, exe)
		return false
	}
	a.setupSink.Emit("INFO", "[update] ✓ actualizado a "+info.Version+". Reiniciando el launcher…")
	relaunch(exe)
	// pequeña pausa para que el nuevo proceso arranque antes de cerrar este.
	time.Sleep(400 * time.Millisecond)
	wailsruntime.Quit(a.ctx)
	return true
}

func fetchLatest() (latestInfo, error) {
	var info latestInfo
	url := fmt.Sprintf("%s/bdpv6-launcher-latest-%s-%s.json", selfUpdateBase, runtime.GOOS, runtime.GOARCH)
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return info, json.NewDecoder(resp.Body).Decode(&info)
}

func downloadVerified(url, dest, wantSHA string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()
	got := hex.EncodeToString(h.Sum(nil))
	if wantSHA != "" && !strings.EqualFold(got, wantSHA) {
		return fmt.Errorf("SHA-256 no coincide (esperado %s, obtenido %s)", wantSHA, got)
	}
	// El binario se baja con os.Create (modo 0644): en Unix queda SIN bit +x y
	// tras el swap el launcher no es ejecutable (relaunch falla con "permission
	// denied"). En Windows el bit de ejecución no aplica. Por eso, en no-Windows,
	// le devolvemos permisos de ejecución al binario auto-actualizado.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dest, 0o755); err != nil {
			return fmt.Errorf("no pude marcar ejecutable el binario nuevo: %w", err)
		}
	}
	return nil
}

// relaunch arranca el exe recién instalado (la app GUI abre su propia ventana).
func relaunch(exe string) {
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	_ = cmd.Start()
}
