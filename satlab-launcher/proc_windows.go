//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hideWindow evita que los procesos hijos (python/jupyter, taskkill, rundll32)
// abran una consola: la experiencia debe ser 100% visual.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// killTree mata el árbol completo de procesos (jupyter-server + kernels).
// taskkill es la herramienta estándar de Windows para esto.
func killTree(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(pid))
	hideWindow(cmd)
	return cmd.Run()
}

// openBrowser abre la URL en el navegador predeterminado.
func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	hideWindow(cmd)
	_ = cmd.Start()
}
