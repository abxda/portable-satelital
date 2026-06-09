package services

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/abxda/bdpv6-launcher/internal/health"
	"github.com/abxda/bdpv6-launcher/internal/logsink"
	"github.com/abxda/bdpv6-launcher/internal/paths"
	"github.com/abxda/bdpv6-launcher/internal/processctl"
)

// Jupyter wraps the bundled Jupyter Lab. The legacy Windows launcher invoked
// jupyter-lab.exe as a Python argument (which is fragile); we use the
// canonical `python -m jupyterlab` form, which works identically on Win and
// Mac. Stdout is scraped for the access URL so the UI can offer a one-click
// "Open Jupyter" button without making the student copy a token by hand.
type Jupyter struct {
	p    *paths.Paths
	port int

	mu        sync.Mutex
	proc      *processctl.Process
	sink      *logsink.Sink
	startedAt time.Time
	exitCode  int
	url       string
}

// urlRegex matches the token-bearing URL Jupyter Server prints at startup.
// Tolerates both /lab?token= and /?token=, and IP / hostname forms.
var urlRegex = regexp.MustCompile(`https?://(?:127\.0\.0\.1|localhost)(?::\d+)?/(?:lab|tree)?\??token=[A-Za-z0-9]+`)

func NewJupyter(p *paths.Paths, port int) *Jupyter {
	if port <= 0 {
		port = 8888
	}
	return &Jupyter{p: p, port: port, sink: logsink.New("jupyter", p.Logs, 2000), exitCode: -1}
}

func (j *Jupyter) ID() string          { return "jupyter" }
func (j *Jupyter) Name() string        { return "Jupyter Lab" }
func (j *Jupyter) Port() int           { return j.port }
func (j *Jupyter) Logs() *logsink.Sink { return j.sink }

// URL returns the cached Jupyter access URL with token if it has been seen
// in the child's output, or an empty string otherwise.
func (j *Jupyter) URL() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.url
}

func (j *Jupyter) Start(ctx context.Context) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.proc != nil && j.proc.Running() {
		return nil
	}

	env := jupyterEnv(j.p)
	args := []string{
		"-m", "jupyterlab",
		fmt.Sprintf("--port=%d", j.port),
		fmt.Sprintf("--notebook-dir=%s", j.p.Notebooks),
		"--no-browser",
		"--ip=127.0.0.1",
	}

	j.sink.Emit("INFO", fmt.Sprintf("Lanzando Jupyter Lab en puerto %d", j.port))
	spec := processctl.Spec{
		Command: j.p.PythonBinary(),
		Args:    args,
		Env:     env,
		Cwd:     j.p.ScriptDir,
		Out: sinkWriter{
			s: j.sink,
			onLine: func(text string) {
				if m := urlRegex.FindString(text); m != "" {
					j.mu.Lock()
					if j.url == "" {
						j.url = m
						j.sink.Emit("INFO", "URL capturada: "+m)
					}
					j.mu.Unlock()
				}
			},
		},
	}
	proc := processctl.New(spec)
	if err := proc.Start(); err != nil {
		j.sink.Emit("ERROR", "Fallo al lanzar Jupyter: "+err.Error())
		return err
	}
	j.proc = proc
	j.startedAt = time.Now()
	j.url = ""
	j.sink.Emit("INFO", fmt.Sprintf("Jupyter arrancado con PID %d", proc.PID()))
	return nil
}

func (j *Jupyter) Stop(ctx context.Context) error {
	j.mu.Lock()
	proc := j.proc
	j.mu.Unlock()
	if proc == nil {
		return nil
	}
	j.sink.Emit("INFO", "Deteniendo Jupyter…")
	err := proc.Stop(10 * time.Second)
	j.mu.Lock()
	j.exitCode = proc.ExitCode()
	j.url = ""
	j.mu.Unlock()
	if err != nil {
		j.sink.Emit("ERROR", "Stop devolvió error: "+err.Error())
	} else {
		j.sink.Emit("INFO", fmt.Sprintf("Jupyter detenido (exit %d)", proc.ExitCode()))
	}
	return err
}

func (j *Jupyter) Status() Status {
	j.mu.Lock()
	proc := j.proc
	since := j.startedAt
	exit := j.exitCode
	url := j.url
	j.mu.Unlock()
	st := Status{ID: j.ID(), Name: j.Name(), Port: j.port, ExitCode: exit}
	if proc != nil && proc.Running() {
		st.Running = true
		st.PID = proc.PID()
		st.Since = since
		probe := health.TCP(fmt.Sprintf("127.0.0.1:%d", j.port), 500*time.Millisecond)
		st.Healthy = probe.Healthy
		st.Detail = probe.Detail
		if url != "" {
			st.Detail = "URL lista"
		}
	} else {
		st.Detail = "detenido"
	}
	return st
}
