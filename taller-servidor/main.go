// TallerServidor — servidor del Taller Offline (habilitador).
//
// Qué hace (TODO el código está a la vista; ~100 líneas):
//  1. Sirve el sitio estático del taller (./sitio) en http://127.0.0.1:8765
//  2. Sirve la carpeta ./mis_datos para que cada estudiante trabaje con SUS
//     propias imágenes y etiquetas: las copia ahí con el Explorador y el
//     cuaderno las encuentra.
//  3. GET /api/mis_datos devuelve la lista de archivos en JSON (para que el
//     cuaderno los enumere).
//
// Qué NO hace: no escribe nada, no acepta subidas, no escucha fuera de
// 127.0.0.1, no toca el registro, no requiere administrador.
//
// Garantías de procedencia (sin firma Authenticode):
//   - código fuente público: github.com/abxda/portable-satelital
//   - compilado en GitHub Actions con artifact attestation
//     (gh attestation verify TallerServidor.exe --repo abxda/portable-satelital)
//   - build reproducible (-trimpath): compílalo tú mismo y compara el SHA-256.
package main

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const puerto = "8765"

func raiz() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}

func main() {
	base := raiz()
	sitio := filepath.Join(base, "sitio")
	datos := filepath.Join(base, "mis_datos")
	_ = os.MkdirAll(datos, 0o755)

	if _, err := os.Stat(filepath.Join(sitio, "lab", "index.html")); err != nil {
		fmt.Println("No encuentro la carpeta sitio\\ junto a este programa.")
		fmt.Println("Descomprime el kit completo y vuelve a intentar.")
		fmt.Scanln()
		os.Exit(1)
	}

	// tipos MIME que el taller necesita sí o sí
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".whl", "application/octet-stream")
	_ = mime.AddExtensionType(".ipynb", "application/json")
	_ = mime.AddExtensionType(".tif", "application/octet-stream")

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(sitio)))
	// mis_datos sin caché: si el alumno copia un archivo nuevo, se ve al instante
	mux.Handle("/mis_datos/", http.StripPrefix("/mis_datos/",
		sinCache(http.FileServer(http.Dir(datos)))))
	mux.HandleFunc("/api/mis_datos", func(w http.ResponseWriter, r *http.Request) {
		type item struct {
			Nombre string `json:"nombre"`
			Bytes  int64  `json:"bytes"`
		}
		var lista []item
		entradas, _ := os.ReadDir(datos)
		for _, e := range entradas {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err == nil {
				lista = append(lista, item{e.Name(), info.Size()})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(lista)
	})

	url := "http://127.0.0.1:" + puerto + "/lab/index.html?path=Taller_ML_Urbano_WASM.ipynb"
	fmt.Println()
	fmt.Println("  Taller Offline sirviendo SOLO en este equipo:")
	fmt.Println("  " + url)
	fmt.Println()
	fmt.Println("  Tus imagenes y etiquetas: copia tus archivos .tif a la carpeta")
	fmt.Println("  mis_datos\\ (junto a este programa) y el cuaderno los vera.")
	fmt.Println()
	fmt.Println("  Para detener: cierra esta ventana.")
	go func() {
		time.Sleep(1200 * time.Millisecond)
		abrirNavegador(url)
	}()
	if err := http.ListenAndServe("127.0.0.1:"+puerto, mux); err != nil {
		fmt.Println("No pude iniciar el servidor:", err)
		fmt.Println("(¿ya hay otro Taller Offline abierto?)")
		fmt.Scanln()
	}
}

func sinCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

func abrirNavegador(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}
