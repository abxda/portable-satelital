package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/abxda/portable-satelital/satlab-launcher/internal/fetch"
	"github.com/abxda/portable-satelital/satlab-launcher/internal/sign"
)

// AppVersion se compara contra el descriptor de self-update publicado en HF.
// Súbela en cada release (y publica el descriptor con la MISMA versión).
const AppVersion = "0.1.0"

// App es el backend que la UI (frontend/dist) invoca vía bindings de Wails.
type App struct {
	ctx context.Context

	mu         sync.Mutex
	installing bool
	jupyter    *exec.Cmd
	jupyterPID int
	labURL     string
	noBrowser  bool // modo headless: no abrir el navegador
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.checkSelfUpdate()
}

func (a *App) shutdown(ctx context.Context) {
	a.StopLab()
}

// --- estado -----------------------------------------------------------------

// State es lo que la UI pinta. Se recalcula on-demand (GetState) y se empuja
// por eventos cuando cambia algo importante.
type State struct {
	LauncherVersion  string `json:"launcherVersion"`
	OSLabel          string `json:"osLabel"`
	Root             string `json:"root"`
	PathHasSpaces    bool   `json:"pathHasSpaces"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	InstalledSHA     string `json:"installedSha"`
	Installing       bool   `json:"installing"`
	Running          bool   `json:"running"`
	LabURL           string `json:"labUrl"`
	KeyFingerprint   string `json:"keyFingerprint"`
}

// root devuelve la carpeta del ejecutable: el laboratorio vive JUNTO al exe
// (decisión del proyecto: portable de verdad, USB-friendly).
func root() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

func pythonExe() string { return filepath.Join(root(), "python", "python.exe") }

// versionInfo es lo que Install deja escrito en .satlab_version: el linaje
// local de QUÉ quedó instalado, de dónde y con qué hash.
type versionInfo struct {
	Version     string `json:"version"`
	SHA256      string `json:"sha256"`
	Source      string `json:"source"`
	InstalledAt string `json:"installedAt"`
}

func readVersionInfo() versionInfo {
	var v versionInfo
	b, err := os.ReadFile(filepath.Join(root(), ".satlab_version"))
	if err == nil {
		_ = json.Unmarshal(b, &v)
	}
	return v
}

func (a *App) GetState() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	vi := readVersionInfo()
	_, errPy := os.Stat(pythonExe())
	return State{
		LauncherVersion:  AppVersion,
		OSLabel:          "Windows · x86-64",
		Root:             root(),
		PathHasSpaces:    strings.ContainsRune(root(), ' '),
		Installed:        errPy == nil,
		InstalledVersion: vi.Version,
		InstalledSHA:     vi.SHA256,
		Installing:       a.installing,
		Running:          a.jupyter != nil,
		LabURL:           a.labURL,
		KeyFingerprint:   sign.Fingerprint(),
	}
}

// --- eventos hacia la UI ------------------------------------------------------

type uiEvent struct {
	Kind   string  `json:"kind"`             // "log" | "progress" | "state" | "update" | "error"
	Level  string  `json:"level,omitempty"`  // log: "ok" | "info" | "warn"
	Msg    string  `json:"msg,omitempty"`    // log / error / update(version)
	Phase  string  `json:"phase,omitempty"`  // progress
	Pct    float64 `json:"pct,omitempty"`    // progress (-1 = indeterminado)
	Detail string  `json:"detail,omitempty"` // progress
}

func (a *App) emit(e uiEvent) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "satlab", e)
	}
}
func (a *App) log(level, msg string)   { a.emit(uiEvent{Kind: "log", Level: level, Msg: msg}) }
func (a *App) pushState()              { a.emit(uiEvent{Kind: "state"}) }
func (a *App) fail(msg string)         { a.emit(uiEvent{Kind: "error", Msg: msg}) }

// --- instalación ---------------------------------------------------------------

// Install arranca la instalación en segundo plano. El avance llega por eventos.
func (a *App) Install() {
	a.mu.Lock()
	if a.installing {
		a.mu.Unlock()
		return
	}
	a.installing = true
	a.mu.Unlock()
	a.pushState()
	go func() {
		err := a.doInstall()
		a.mu.Lock()
		a.installing = false
		a.mu.Unlock()
		if err != nil {
			a.fail(err.Error())
		}
		a.pushState()
	}()
}

func (a *App) doInstall() error {
	dst := root()

	a.emit(uiEvent{Kind: "progress", Phase: "Leyendo el catálogo…", Pct: -1})
	a.log("info", "Consultando el catálogo de distribuciones…")
	raw, sig, err := fetch.FetchManifestRaw()
	if err != nil {
		return fmt.Errorf("no pude leer el catálogo: %v. Revisa tu conexión a internet e inténtalo de nuevo", err)
	}
	if err := signVerify(raw, sig); err != nil {
		return err
	}
	a.log("ok", "Catálogo auténtico: firma Ed25519 verificada (llave "+signFingerprint()+")")

	man := fetch.Parse(raw)
	key := runtime.GOOS + "-" + runtime.GOARCH + "-portable"
	entry, err := man.Resolve(key)
	if err != nil {
		return fmt.Errorf("tu plataforma (%s) aún no tiene una distribución publicada: %v", key, err)
	}

	sizeLbl := entry.Size
	if sizeLbl == "" {
		sizeLbl = "~1 GB"
	}
	a.log("info", fmt.Sprintf("Distribución %s (%s). Descargando de %s…", entry.Version, sizeLbl, entry.DownloadURL()))
	tarball := filepath.Join(dst, ".satlab-download.tar.gz")
	phase := fmt.Sprintf("Descargando el laboratorio (%s)…", sizeLbl)
	err = fetch.Download(entry, tarball, func(done, total int64) {
		pct := -1.0
		detail := humanBytes(done)
		if total > 0 {
			pct = float64(done) / float64(total) * 100
			detail = humanBytes(done) + " / " + humanBytes(total)
		}
		a.emit(uiEvent{Kind: "progress", Phase: phase, Pct: pct, Detail: detail})
	})
	if err != nil {
		return fmt.Errorf("falló la descarga: %v", err)
	}
	a.log("ok", "Integridad verificada: SHA-256 coincide ("+entry.SHA256[:16]+"…)")

	a.emit(uiEvent{Kind: "progress", Phase: "Descomprimiendo (puede tardar varios minutos)…", Pct: -1})
	err = fetch.ExtractTarGzProgress(tarball, dst, func(files int, b int64) {
		if files%250 == 0 {
			a.emit(uiEvent{Kind: "progress", Phase: "Descomprimiendo (puede tardar varios minutos)…", Pct: -1,
				Detail: fmt.Sprintf("%d archivos · %s", files, humanBytes(b))})
		}
	})
	if err != nil {
		os.Remove(tarball)
		return fmt.Errorf("falló la descompresión: %v", err)
	}
	os.Remove(tarball)

	// Honestidad del "paso completado": no basta el exit code — verifica que el
	// resultado real exista antes de cantar victoria.
	if _, err := os.Stat(pythonExe()); err != nil {
		return fmt.Errorf("la extracción terminó pero no encuentro python\\python.exe — instalación incompleta")
	}

	vi := versionInfo{
		Version:     entry.Version,
		SHA256:      entry.SHA256,
		Source:      entry.DownloadURL(),
		InstalledAt: time.Now().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(vi, "", "  ")
	_ = os.WriteFile(filepath.Join(dst, ".satlab_version"), b, 0o644)

	a.log("ok", "Laboratorio instalado y verificado. Listo para abrir.")
	return nil
}

// signVerify / signFingerprint: indirection mínima para mantener app.go legible.
func signVerify(data, sigB64 []byte) error { return sign.Verify(data, sigB64) }
func signFingerprint() string              { return sign.Fingerprint() }

// --- Jupyter -------------------------------------------------------------------

// OpenLab lanza JupyterLab (si no corre ya) y abre el navegador. Sin token:
// la config del portable lo desactiva y amarra el bind a 127.0.0.1.
func (a *App) OpenLab() {
	a.mu.Lock()
	if a.jupyter != nil {
		url := a.labURL
		a.mu.Unlock()
		openBrowser(url)
		return
	}
	a.mu.Unlock()
	go func() {
		if err := a.startJupyter(); err != nil {
			a.fail(err.Error())
		}
		a.pushState()
	}()
}

func (a *App) startJupyter() error {
	rt := root()
	py := pythonExe()
	if _, err := os.Stat(py); err != nil {
		return fmt.Errorf("no encuentro el laboratorio instalado (python\\python.exe). Instálalo primero")
	}
	port, err := freePort(8888, 8899)
	if err != nil {
		return err
	}

	a.emit(uiEvent{Kind: "progress", Phase: "Arrancando JupyterLab…", Pct: -1})
	notebooks := filepath.Join(rt, "notebooks")
	_ = os.MkdirAll(notebooks, 0o755)
	cfgDir := filepath.Join(rt, "jupyter-config")
	dataDir := filepath.Join(rt, "jupyter-data")

	cmd := exec.Command(py, "-m", "jupyterlab",
		fmt.Sprintf("--port=%d", port), "--no-browser",
		"--config="+filepath.Join(cfgDir, "jupyter_server_config.py"))
	cmd.Dir = notebooks // root_dir de Jupyter = carpeta de cuadernos

	env := os.Environ()
	pyDir := filepath.Join(rt, "python")
	// PATH con python\ y python\Scripts\ al frente: así `!pip`, `!python` y los
	// entry-points instalados por los alumnos (%pip install …) funcionan dentro
	// de los notebooks.
	env = append(env,
		"PATH="+pyDir+";"+filepath.Join(pyDir, "Scripts")+";"+os.Getenv("PATH"),
		"JUPYTER_CONFIG_DIR="+cfgDir,
		"JUPYTER_DATA_DIR="+dataDir,
		"JUPYTER_RUNTIME_DIR="+filepath.Join(dataDir, "runtime"),
		"PYTHONIOENCODING=utf-8",
	)
	// GDAL/PROJ: normalmente las wheels se resuelven solas; si los datos están
	// donde se espera, los exportamos explícitos (cinturón y tirantes).
	sitePk := filepath.Join(pyDir, "Lib", "site-packages")
	if p := filepath.Join(sitePk, "pyproj", "proj_dir", "share", "proj"); isDir(p) {
		env = append(env, "PROJ_DATA="+p)
	}
	if p := filepath.Join(sitePk, "rasterio", "gdal_data"); isDir(p) {
		env = append(env, "GDAL_DATA="+p)
	}
	cmd.Env = env

	// La salida de Jupyter queda en un log junto al laboratorio (diagnóstico).
	logf, err := os.OpenFile(filepath.Join(rt, "jupyter.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err == nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no pude lanzar JupyterLab: %v", err)
	}

	// Espera a que el puerto responda (hasta 90 s: el primer arranque compila
	// cachés y puede tardar).
	deadline := time.Now().Add(90 * time.Second)
	up := false
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			up = true
			break
		}
		// ¿murió el proceso? (puerto nunca va a abrir)
		if cmd.ProcessState != nil {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if !up {
		_ = killTree(cmd.Process.Pid)
		return fmt.Errorf("JupyterLab no respondió a tiempo. Revisa jupyter.log junto al programa")
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/lab", port)
	a.mu.Lock()
	a.jupyter = cmd
	a.jupyterPID = cmd.Process.Pid
	a.labURL = url
	a.mu.Unlock()

	// Si Jupyter muere por su cuenta, refleja el estado en la UI.
	go func() {
		_ = cmd.Wait()
		a.mu.Lock()
		if a.jupyter == cmd {
			a.jupyter = nil
			a.labURL = ""
		}
		a.mu.Unlock()
		a.pushState()
	}()

	a.log("ok", "JupyterLab corriendo SOLO en tu equipo (127.0.0.1, sin acceso desde la red).")
	if !a.noBrowser {
		openBrowser(url)
	}
	return nil
}

// StopLab detiene Jupyter y sus kernels (árbol completo de procesos).
func (a *App) StopLab() {
	a.mu.Lock()
	pid := a.jupyterPID
	running := a.jupyter != nil
	a.jupyter = nil
	a.labURL = ""
	a.mu.Unlock()
	if running && pid > 0 {
		_ = killTree(pid)
		a.log("info", "Laboratorio detenido.")
	}
	a.pushState()
}

// OpenFolder abre la carpeta del laboratorio en el Explorador.
func (a *App) OpenFolder() {
	cmd := exec.Command("explorer", root())
	_ = cmd.Start()
}

// --- helpers ---------------------------------------------------------------------

func freePort(from, to int) (int, error) {
	for p := from; p <= to; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			l.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no encontré un puerto libre entre %d y %d", from, to)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func humanBytes(n int64) string {
	const u = 1024
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(u), 0
	for x := n / u; x >= u; x /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
