// Package paths resolves the on-disk layout of the BDP distribution.
//
// The launcher binary is expected to live at the root of a BDP distribution,
// next to the service folders (common_jdk/, hadoop/, kafka_kraft/,
// elasticsearch/, python/, spark/, notebooks/, data/, logs/). ScriptDir is
// the directory containing the running binary, computed once at process
// start. All service paths are derived from it.
package paths

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Paths is the immutable, fully resolved layout.
type Paths struct {
	ScriptDir   string
	CommonJDK   string
	Hadoop      string
	HadoopConf  string
	Kafka       string
	Elastic     string
	Python      string
	Spark       string
	Notebooks   string
	Data        string
	Logs        string
	StateFile   string
}

// Detect resolves Paths based on the location of the running executable.
// Fallback: current working directory. Callers should not panic on an
// incomplete distribution — use Validate to surface missing folders to
// the user via the UI.
func Detect() *Paths {
	dir := executableDir()
	return &Paths{
		ScriptDir:  dir,
		CommonJDK:  detectJavaHome(filepath.Join(dir, "common_jdk")),
		Hadoop:     filepath.Join(dir, "hadoop"),
		HadoopConf: filepath.Join(dir, "hadoop", "etc", "hadoop"),
		Kafka:      filepath.Join(dir, "kafka_kraft"),
		Elastic:    filepath.Join(dir, "elasticsearch"),
		Python:     filepath.Join(dir, "python"),
		Spark:      filepath.Join(dir, "spark"),
		Notebooks:  filepath.Join(dir, "notebooks"),
		Data:       filepath.Join(dir, "data"),
		Logs:       filepath.Join(dir, "logs"),
		StateFile:  filepath.Join(dir, ".bdp_state.json"),
	}
}

// detectJavaHome resolves the actual JAVA_HOME inside common_jdk/. On
// Windows / Linux the layout is flat (common_jdk/bin/java). On macOS,
// JDK distributions ship as bundles: common_jdk/<name>.jdk/Contents/Home/
// is the real JAVA_HOME, and bin/java lives one level deeper. Without
// this unwrap, every service that needs Java fails on Mac because
// JAVA_HOME points at the wrapper dir and bin/java isn't found.
func detectJavaHome(commonJDK string) string {
	if _, err := os.Stat(filepath.Join(commonJDK, "bin", "java")); err == nil {
		return commonJDK
	}
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(filepath.Join(commonJDK, "bin", "java.exe")); err == nil {
			return commonJDK
		}
	}
	matches, _ := filepath.Glob(filepath.Join(commonJDK, "*.jdk", "Contents", "Home"))
	if len(matches) > 0 {
		return matches[0]
	}
	return commonJDK // best effort; Validate will surface the failure
}

// Validate reports whether the expected service folders exist next to the
// binary. Returns (true, nil) on a healthy layout, (false, missing) otherwise.
// Notebooks/data/logs are not required (they get auto-created by setup).
func (p *Paths) Validate() (bool, []string) {
	required := map[string]string{
		"common_jdk":    p.CommonJDK,
		"hadoop":        p.Hadoop,
		"kafka_kraft":   p.Kafka,
		"elasticsearch": p.Elastic,
		"python":        p.Python,
	}
	var missing []string
	for name, path := range required {
		if !isDir(path) {
			missing = append(missing, name)
		}
	}
	return len(missing) == 0, missing
}

// ServicePaths returns a stable map of {service: absolute path}, useful for
// the frontend to display the detected layout. Returned even if some entries
// are missing; the UI cross-references with Validate().
func (p *Paths) ServicePaths() map[string]string {
	return map[string]string{
		"common_jdk":    p.CommonJDK,
		"hadoop":        p.Hadoop,
		"hadoop_conf":   p.HadoopConf,
		"kafka_kraft":   p.Kafka,
		"elasticsearch": p.Elastic,
		"python":        p.Python,
		"spark":         p.Spark,
		"notebooks":     p.Notebooks,
		"data":          p.Data,
		"logs":          p.Logs,
	}
}

// NamenodeFormatted reports whether HDFS NameNode appears to have been
// formatted (the standard marker file exists). Used by the first-run wizard
// to decide whether to prompt for setup.
func (p *Paths) NamenodeFormatted() bool {
	marker := filepath.Join(p.Data, "hdfs", "namenode", "current", "VERSION")
	_, err := os.Stat(marker)
	return err == nil
}

// NamenodeState is what the dashboard reports about the on-disk HDFS state.
type NamenodeState string

const (
	// NamenodeEmpty: namenode dir does not exist or has no current/. Means
	// the user has never run the format step (or just cleaned the data dir).
	NamenodeEmpty NamenodeState = "empty"
	// NamenodeFormattedOK: current/VERSION exists. Hadoop will accept this
	// dir as a valid formatted namenode.
	NamenodeFormattedOK NamenodeState = "formatted"
	// NamenodeCorrupted: current/ has stale files (edits_inprogress_*,
	// in_use.lock, …) but no VERSION. This usually means a previous run
	// was force-killed before HDFS could checkpoint. The data is
	// unrecoverable; the user needs to reformat (which is destructive but
	// the right move because the namespace metadata is already gone).
	NamenodeCorruptedSt NamenodeState = "corrupted"
)

// NamenodeStateOf inspects the namenode directory and classifies it.
func (p *Paths) NamenodeStateOf() NamenodeState {
	curr := filepath.Join(p.Data, "hdfs", "namenode", "current")
	if _, err := os.Stat(curr); err != nil {
		// current/ does not exist — maybe the parent does (we created the
		// data dirs at setup) but nothing inside. Either way: empty.
		return NamenodeEmpty
	}
	version := filepath.Join(curr, "VERSION")
	if _, err := os.Stat(version); err == nil {
		return NamenodeFormattedOK
	}
	// current/ exists but VERSION does not. Check if anything inside —
	// an empty current/ is just "fresh", not corrupted.
	entries, err := os.ReadDir(curr)
	if err != nil || len(entries) == 0 {
		return NamenodeEmpty
	}
	return NamenodeCorruptedSt
}

// KafkaFormatted reports whether Kafka KRaft storage has been formatted —
// the test is the presence of meta.properties inside the log dir defined by
// server.properties. Kafka resolves log.dirs=./data/data_kraft relative to
// the cwd we pass on launch (== paths.Kafka), so the on-disk location is
// kafka_kraft/data/data_kraft/meta.properties, not the top-level data/.
func (p *Paths) KafkaFormatted() bool {
	marker := filepath.Join(p.Kafka, "data", "data_kraft", "meta.properties")
	_, err := os.Stat(marker)
	return err == nil
}

// HadoopConfigGenerated reports whether the Hadoop XMLs have been generated
// by the setup wizard. We test core-site.xml because the wizard writes both
// XMLs in one shot; if one is present so is the other.
func (p *Paths) HadoopConfigGenerated() bool {
	_, err := os.Stat(p.CoreSiteXML())
	return err == nil
}

// JavaBinary returns the absolute path to the JDK java executable bundled
// with the distribution. Honours OS-specific naming.
func (p *Paths) JavaBinary() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(p.CommonJDK, "bin", "java.exe")
	}
	return filepath.Join(p.CommonJDK, "bin", "java")
}

// HdfsCommand returns the launcher script for the HDFS daemons.
func (p *Paths) HdfsCommand() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(p.Hadoop, "bin", "hdfs.cmd")
	}
	return filepath.Join(p.Hadoop, "bin", "hdfs")
}

// KafkaStartCommand returns the kafka-server-start script for the host OS.
func (p *Paths) KafkaStartCommand() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(p.Kafka, "bin", "windows", "kafka-server-start.bat")
	}
	return filepath.Join(p.Kafka, "bin", "kafka-server-start.sh")
}

// KafkaStorageCommand returns the kafka-storage script for the host OS.
func (p *Paths) KafkaStorageCommand() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(p.Kafka, "bin", "windows", "kafka-storage.bat")
	}
	return filepath.Join(p.Kafka, "bin", "kafka-storage.sh")
}

// KafkaConfig returns the path to the KRaft server.properties file.
func (p *Paths) KafkaConfig() string {
	return filepath.Join(p.Kafka, "config", "kraft", "server.properties")
}

// ElasticBinary returns the elasticsearch launcher for the host OS.
func (p *Paths) ElasticBinary() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(p.Elastic, "bin", "elasticsearch.bat")
	}
	return filepath.Join(p.Elastic, "bin", "elasticsearch")
}

// PythonBinary returns the embedded Python interpreter for the host OS.
func (p *Paths) PythonBinary() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(p.Python, "python.exe")
	}
	return filepath.Join(p.Python, "bin", "python3")
}

// CoreSiteXML / HdfsSiteXML are the Hadoop config files generated by setup.
func (p *Paths) CoreSiteXML() string {
	return filepath.Join(p.HadoopConf, "core-site.xml")
}
func (p *Paths) HdfsSiteXML() string {
	return filepath.Join(p.HadoopConf, "hdfs-site.xml")
}

// hadoopIdentRe matches the active 'set HADOOP_IDENT_STRING=...' line in
// hadoop-env.cmd (NOT the @rem-commented one), without consuming the trailing
// CRLF so we preserve Windows line endings on rewrite.
var hadoopIdentRe = regexp.MustCompile(`(?mi)^[ \t]*set HADOOP_IDENT_STRING=[^\r\n]*`)

// EnsureHadoopIdentString fija HADOOP_IDENT_STRING en hadoop-env.cmd a un valor
// SIN espacios ("bdp"). Por defecto Hadoop usa %USERNAME%; si el usuario de
// Windows tiene espacios ("Juan PC"), hadoop-config.cmd construye
// -Dhadoop.id.str=Juan PC SIN comillas, Java parte el espacio y toma "PC" como
// clase principal -> "Could not find or load main class PC", y el formateo del
// NameNode falla en silencio. Un valor fijo sin espacios elimina el problema
// para CUALQUIER nombre de usuario. Idempotente: solo reescribe si hace falta.
// Si el archivo no existe (no es Windows), es un no-op silencioso.
func (p *Paths) EnsureHadoopIdentString() (bool, error) {
	envFile := filepath.Join(p.HadoopConf, "hadoop-env.cmd")
	data, err := os.ReadFile(envFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if strings.Contains(string(data), "set HADOOP_IDENT_STRING=bdp") {
		return false, nil // ya parcheado
	}
	if !hadoopIdentRe.Match(data) {
		return false, nil // no está la línea esperada; no tocamos nada
	}
	patched := hadoopIdentRe.ReplaceAll(data, []byte("set HADOOP_IDENT_STRING=bdp"))
	if err := os.WriteFile(envFile, patched, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// --- internals ---

func executableDir() string {
	exe, err := os.Executable()
	if err == nil {
		// Resolve symlinks so we land on the real on-disk location.
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		return unwrapAppBundle(filepath.Dir(exe))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// unwrapAppBundle maps the directory containing the running binary to the
// BDP distribution root. On macOS the launcher ships as a .app bundle, so the
// binary lives at <root>/<Name>.app/Contents/MacOS/<bin> while the distro
// folders (common_jdk/, hadoop/, …) are siblings of the .app. We unwrap the
// three bundle levels (MacOS → Contents → <Name>.app) to land on <root>.
// Without this every service path resolves inside Contents/MacOS/ and
// fork/exec fails with "no such file or directory". On non-darwin hosts, or
// when the binary is not inside a .app (e.g. `wails dev`, `go test`), the
// directory is returned unchanged.
func unwrapAppBundle(dir string) string {
	if runtime.GOOS != "darwin" || filepath.Base(dir) != "MacOS" {
		return dir
	}
	contents := filepath.Dir(dir)
	if filepath.Base(contents) != "Contents" {
		return dir
	}
	appDir := filepath.Dir(contents)
	if filepath.Ext(appDir) != ".app" {
		return dir
	}
	return filepath.Dir(appDir)
}
