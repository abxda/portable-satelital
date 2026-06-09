// Package fetch descarga, verifica y descomprime el laboratorio desde el
// dataset público de Hugging Face. Usa SOLO la stdlib de Go (net/http,
// crypto/sha256, archive/tar, compress/gzip): no depende de curl ni tar
// externos, así el binario es autosuficiente y no dispara antivirus al lanzar
// procesos hijos. (Patrón heredado del meta-launcher del Big Data lab.)
package fetch

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultManifestURL es el ÚNICO dato cableado: dónde vive el manifiesto.
// Todo lo demás (archivos, URLs, hashes, versiones) se define DENTRO del
// manifiesto, que además viaja FIRMADO (ver internal/sign): el launcher
// rechaza un manifiesto cuya firma Ed25519 no verifique contra la llave
// pública embebida en este binario.
const DefaultManifestURL = "https://huggingface.co/datasets/abxda/portable-satelital/resolve/main/manifest.txt"

// ManifestURL devuelve la URL efectiva del manifiesto: el override de entorno
// SATLAB_MANIFEST_URL (para pruebas) o la URL por defecto.
func ManifestURL() string {
	if v := os.Getenv("SATLAB_MANIFEST_URL"); v != "" {
		return v
	}
	return DefaultManifestURL
}

func manifestBase() string {
	u := ManifestURL()
	if i := strings.LastIndexByte(u, '/'); i >= 0 {
		return u[:i]
	}
	return u
}

// Manifest mapea clave -> valor del manifest.txt.
type Manifest map[string]string

// FetchBytes baja una URL completa a memoria (manifiesto, firmas, descriptores).
func FetchBytes(url string) ([]byte, error) {
	body, _, err := httpGetLen(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

// FetchManifestRaw baja el manifiesto y su firma desacoplada (.sig).
// La verificación criptográfica la hace el llamador (internal/sign) ANTES de
// parsear o usar cualquier valor.
func FetchManifestRaw() (raw []byte, sig []byte, err error) {
	raw, err = FetchBytes(ManifestURL())
	if err != nil {
		return nil, nil, err
	}
	sig, err = FetchBytes(ManifestURL() + ".sig")
	if err != nil {
		return nil, nil, fmt.Errorf("no pude leer la firma del catálogo (%s.sig): %w", ManifestURL(), err)
	}
	return raw, sig, nil
}

// Parse convierte el texto del manifiesto (líneas "clave=valor"; # comentario).
func Parse(data []byte) Manifest {
	m := Manifest{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '='); i > 0 {
			m[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
	return m
}

// Entry resuelve los datos de una distribución para una clave base
// (p.ej. "windows-amd64-portable").
type Entry struct {
	Base    string
	File    string // nombre de archivo destino
	URL     string // URL absoluta (opcional; si falta se deriva del manifiesto)
	SHA256  string
	Version string // versión humana del stack (opcional)
	Size    string // etiqueta informativa "~1.2 GB" (opcional)
}

func (m Manifest) Resolve(base string) (Entry, error) {
	e := Entry{
		Base:    base,
		File:    m[base+".file"],
		URL:     m[base+".url"],
		SHA256:  m[base+".sha256"],
		Version: m[base+".version"],
		Size:    m[base+".size"],
	}
	if e.File == "" || e.SHA256 == "" {
		return e, fmt.Errorf("el catálogo no tiene .file/.sha256 para %q", base)
	}
	if len(e.SHA256) != 64 {
		return e, fmt.Errorf("sha256 malformado en el catálogo para %q", base)
	}
	return e, nil
}

func (e Entry) DownloadURL() string {
	if e.URL != "" {
		return e.URL
	}
	return manifestBase() + "/" + e.File
}

// ProgressFn recibe (bytesDescargados, bytesTotales). total=0 si es desconocido.
type ProgressFn func(done, total int64)

// Download trae e a destPath con progreso. Verifica el SHA-256 al terminar y
// BORRA el archivo si no coincide (nunca queda un artefacto no verificado).
func Download(e Entry, destPath string, prog ProgressFn) error {
	body, total, err := httpGetLen(e.DownloadURL())
	if err != nil {
		return err
	}
	defer body.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	h := sha256.New()
	pw := &progWriter{total: total, prog: prog, last: time.Now()}
	_, copyErr := io.Copy(io.MultiWriter(out, h, pw), body)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(destPath)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(destPath)
		return closeErr
	}
	if prog != nil {
		prog(pw.done, total)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, e.SHA256) {
		os.Remove(destPath)
		return fmt.Errorf("SHA-256 no coincide: esperado %s, obtenido %s (descarga corrupta o alterada; el archivo fue eliminado)", e.SHA256, got)
	}
	return nil
}

// VerifySHA256File calcula el sha256 de un archivo y lo compara (para
// re-verificar el binario del self-update tras escribirlo a disco).
func VerifySHA256File(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("SHA-256 no coincide (esperado %s, obtenido %s)", want, got)
	}
	return nil
}

// ExtractTarGzProgress descomprime un .tar.gz en destDir con callback de
// avance. Protege contra path traversal (entradas con .. o rutas absolutas).
// Maneja archivos regulares, directorios, symlinks y hardlinks; en Windows un
// symlink que falle por privilegios se omite sin abortar (los tars de Windows
// no usan symlinks).
func ExtractTarGzProgress(archivePath, destDir string, onProgress func(files int, bytes int64)) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	cleanDest := filepath.Clean(destDir)
	var files int
	var bytes int64
	tick := func() {
		if onProgress != nil {
			onProgress(files, bytes)
		}
	}
	for {
		hd, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, hd.Name)
		if !strings.HasPrefix(filepath.Clean(target), cleanDest+string(os.PathSeparator)) &&
			filepath.Clean(target) != cleanDest {
			return fmt.Errorf("entrada insegura en el tar: %q", hd.Name)
		}
		switch hd.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hd.Mode)&0o777)
			if err != nil {
				return err
			}
			n, err := io.Copy(w, tr)
			w.Close()
			if err != nil {
				return err
			}
			bytes += n
			files++
			tick()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(hd.Linkname, target); err != nil {
				continue
			}
			files++
			tick()
		case tar.TypeLink:
			os.Remove(target)
			if err := os.Link(filepath.Join(destDir, hd.Linkname), target); err != nil {
				continue
			}
			files++
			tick()
		}
	}
	return nil
}

// --- helpers HTTP ---

func httpGetLen(url string) (io.ReadCloser, int64, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "satlab-launcher")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("HTTP %d al pedir %s", resp.StatusCode, url)
	}
	return resp.Body, resp.ContentLength, nil
}

type progWriter struct {
	done  int64
	total int64
	prog  ProgressFn
	last  time.Time
}

func (p *progWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.done += int64(n)
	if p.prog != nil && time.Since(p.last) > 100*time.Millisecond {
		p.prog(p.done, p.total)
		p.last = time.Now()
	}
	return n, nil
}
