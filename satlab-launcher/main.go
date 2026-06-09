// SatLab — launcher visual ÚNICO del Laboratorio Satelital Portable.
//
// Un solo exe: doble clic → ventana. Si el laboratorio no está instalado, lo
// descarga del catálogo FIRMADO (Ed25519, llave embebida), verifica SHA-256,
// extrae junto al exe y queda listo. Si ya está, un botón abre JupyterLab
// (sin token, SOLO 127.0.0.1) en el navegador. Al cerrar la ventana se apaga
// Jupyter y sus kernels.
//
// Por qué Wails/WebView2 y no otra cosa: produce un PE normal (sin trucos de
// proceso que asusten al EDR de equipos administrados) y WebView2 viene
// preinstalado en Windows 10/11. Patrón heredado del Big Data lab.
package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--headless") {
		os.Exit(runHeadless(os.Args[1]))
	}
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "Laboratorio Satelital Portable",
		Width:  900,
		Height: 660,
		MinWidth:  760,
		MinHeight: 560,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,
		OnShutdown:  app.shutdown,
		Bind:        []interface{}{app},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			// Pulcritud: el caché de WebView2 vive DENTRO de la carpeta del
			// laboratorio (no en %APPDATA%). Borrar la carpeta = desinstalar
			// todo, sin rastros en el perfil del usuario.
			WebviewUserDataPath: filepath.Join(root(), ".webview2"),
		},
	})
	if err != nil {
		panic(err)
	}
}
