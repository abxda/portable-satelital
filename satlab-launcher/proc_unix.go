//go:build !windows

package main

import (
	"os/exec"
	"runtime"
	"syscall"
)

// hideWindow en Unix no oculta nada (no hay consolas emergentes), pero pone al
// hijo en su PROPIO grupo de procesos: así killTree puede matar a Jupyter y a
// todos sus kernels de un solo golpe.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree mata el grupo de procesos completo (jupyter-server + kernels).
func killTree(pid int) error {
	// pid negativo = todo el grupo (el hijo es líder de grupo por Setpgid).
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		return syscall.Kill(pid, syscall.SIGTERM)
	}
	return nil
}

// openBrowser abre la URL en el navegador predeterminado.
func openBrowser(url string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else {
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
