# Cómo verificar este software (guía para áreas de TI y seguridad)

**Producto:** Laboratorio Satelital Portable (`SatLab.exe`) — entorno educativo de análisis
de imágenes satelitales (JupyterLab + stack geoespacial de Python), usado en cursos para
servidores públicos.
**Autor:** Dr. Abel Coronado ([@abxda](https://github.com/abxda)).
**Código fuente:** <https://github.com/abxda/portable-satelital> (100% público y auditable).

Este documento explica, en orden de profundidad, cómo comprobar que lo que su personal
descargó es exactamente lo que se publicó, de dónde proviene cada pieza, y qué hace (y qué
NO hace) en el equipo.

---

## 0) Qué es, dónde se descarga y cómo se ejecuta

**Qué es.** Un **laboratorio de análisis de imágenes satelitales** para capacitación: un
solo archivo `SatLab.exe` que, al abrirse, levanta un **JupyterLab** con Python y las
bibliotecas geoespaciales ya listas. No es un instalador: es una aplicación portable que
vive en su propia carpeta.

**Dónde se descarga (liga directa y oficial).**

- **Descarga directa del ejecutable (Windows):**
  <https://github.com/abxda/portable-satelital/releases/latest/download/SatLab.exe>
- Página de todas las versiones y archivos (Linux, hashes, notas):
  <https://github.com/abxda/portable-satelital/releases/latest>

> Solo descargue desde estas ligas de **GitHub Releases** del repositorio oficial
> `abxda/portable-satelital`. No lo distribuya por correo ni desde otros sitios.

**Cómo se ejecuta (3 pasos).**

1. Descargue `SatLab.exe` y colóquelo en una carpeta vacía (p. ej. `D:\SatLab\`).
2. Doble clic. La primera vez descarga el entorno (~1 GB) y lo deja junto al `.exe`;
   Windows SmartScreen mostrará un aviso (es un binario sin firma comercial — ver §1 y §4):
   *Más información → Ejecutar de todas formas*.
3. Se abre una ventana con JupyterLab listo para trabajar. Para quitarlo, **borre la
   carpeta**: no deja nada fuera de ella.

El resto de este documento es para su **área de TI / seguridad**: explica con qué evidencia
verificable se comprueba la autenticidad e integridad del binario, su comportamiento de red
y por qué pueden aparecer (y cómo desmentir) falsos positivos heurísticos.

---

## 1) Qué hace el software (resumen para evaluación de riesgo)

- `SatLab.exe` es una aplicación de escritorio (Windows, WebView2) **sin instalador**: no
  toca el registro, no requiere privilegios de administrador, no instala servicios ni
  drivers. Todo vive en la carpeta donde se coloca el `.exe`.
- Al primer uso descarga una distribución de Python portable (~1 GB) desde un repositorio
  público de Hugging Face, **verifica su integridad** (ver §3) y la extrae junto al `.exe`.
- Después lanza **JupyterLab amarrado exclusivamente a 127.0.0.1** (loopback). No abre
  puertos hacia la red local ni acepta conexiones externas. No expone servicios.
- Conexiones de red salientes del launcher: únicamente a `huggingface.co` (catálogo y
  descarga de la distribución) y las que el usuario haga desde sus cuadernos.
- Desinstalación = borrar la carpeta. No deja rastro fuera de ella (los datos de Jupyter
  también viven dentro de la carpeta del laboratorio).
- El binario **no está firmado con Authenticode** (proyecto educativo sin certificado
  comercial); SmartScreen mostrará el aviso estándar. La autenticidad se verifica con los
  mecanismos de abajo, que son criptográficamente más fuertes que la reputación de
  SmartScreen.

## 2) Verificar que el binario proviene del código fuente público (procedencia / SLSA)

Cada release de `SatLab.exe` se compila en **GitHub Actions** (no en una máquina personal)
y se publica con **artifact attestation** de GitHub: una prueba criptográfica de qué
repositorio, commit y workflow produjeron el binario.

```bash
# Requiere GitHub CLI (https://cli.github.com)
gh attestation verify SatLab.exe --repo abxda/portable-satelital
```

Salida esperada: `✓ Verification succeeded` con el commit y el workflow que lo construyó.
Esto garantiza que el exe corresponde al código fuente público, sin modificaciones.

## 3) Cadena de integridad de lo que se descarga en tiempo de ejecución

1. El launcher trae **embebida** la llave pública Ed25519 del proyecto (visible en el
   fuente: `satlab-launcher/internal/sign/sign.go`).
2. El catálogo (`manifest.txt`) que dice qué descargar viaja **firmado** (`manifest.txt.sig`).
   Si la firma no verifica, el launcher se niega a descargar. Un atacante que comprometiera
   el hosting no puede redirigir las descargas sin poseer la llave privada.
3. Cada distribución descargada se verifica contra su **SHA-256** declarado en el catálogo
   firmado; si no coincide, el archivo se elimina y la instalación aborta.
4. Las bibliotecas de Python dentro de la distribución se instalaron con
   `pip install --require-hashes` contra `requirements.lock` (en este repositorio): cada
   wheel de PyPI quedó verificada contra su hash en el momento de construcción.
5. El intérprete de Python proviene de
   [python-build-standalone](https://github.com/astral-sh/python-build-standalone)
   (proyecto de Astral, licencia PSF), verificado contra su `SHA256SUMS` oficial al
   construir. **No se usa Anaconda ni conda** (sin implicaciones de licencia comercial).

Hashes oficiales de cada release: publicados en la página de
[Releases](https://github.com/abxda/portable-satelital/releases) (`SHA256SUMS.txt`).
Verificación manual:

```powershell
Get-FileHash SatLab.exe -Algorithm SHA256   # comparar contra SHA256SUMS.txt del release
```

## 4) Análisis antivirus independiente — y los falsos positivos esperables

Cada release se somete a [VirusTotal](https://www.virustotal.com) y el enlace se incluye en
las notas del release. Puede re-subirlo usted mismo: el SHA-256 debe coincidir.

**Qué esperar del reporte (transparencia total):** los motores tradicionales de firmas
(Kaspersky, ESET, Sophos, Symantec, BitDefender, TrendMicro, McAfee, Elastic, etc.) marcan
el binario **limpio**. Un puñado de motores puramente *heurísticos/ML* puede marcarlo como
sospechoso con etiquetas genéricas (p. ej. `Trojan:Win32/Wacatac.B!ml` de Microsoft — la
terminación `!ml` indica veredicto de *machine learning*, no firma de malware conocida).
Es el falso positivo documentado y clásico para **ejecutables Go sin firma Authenticode**:
binario nuevo sin reputación + símbolos de depuración eliminados. Cómo desmentirlo con
evidencia verificable, en orden de fuerza:

1. La **attestation** (§2): prueba criptográfica de que el exe salió del código fuente
   público, compilado en GitHub — un troyano no puede producirla.
2. El **código fuente completo** está publicado; la lógica de red es auditable: las únicas
   conexiones propias son a `huggingface.co` (catálogo/descarga) y `github.com`
   (auto-actualización). Los dominios de Microsoft/Akamai/DigiCert que aparecen en
   sandboxes provienen de WebView2 y del propio Windows (revocación de certificados,
   recursos de idioma), no de este programa.
3. Reportamos cada release como **falso positivo a Microsoft** (WDSI) y a los demás
   proveedores; estos veredictos ML suelen limpiarse en días.

Si su política institucional exige cero alertas heurísticas, la alternativa es esperar la
versión con firma Authenticode (en evaluación) o autorizar el hash específico del release
(allowlisting por SHA-256), que es criptográficamente más fuerte que la reputación.

## 5) Resumen de la postura de seguridad

| Control | Mecanismo |
|---|---|
| Procedencia del binario | GitHub Actions + artifact attestation (verificable con `gh`) |
| Integridad del catálogo | Firma Ed25519, llave pública embebida en el binario |
| Integridad de descargas | SHA-256 obligatorio (falla = se borra y aborta) |
| Integridad del stack Python | `pip --require-hashes` + lock auditable en el repo |
| Origen del intérprete | python-build-standalone (Astral), verificado contra SHA256SUMS |
| Exposición de red | Jupyter solo en 127.0.0.1; sin servicios, sin puertos abiertos |
| Privilegios | Ninguno: corre como usuario estándar, sin instalador, sin registro |
| Reversibilidad | Borrar la carpeta elimina todo |

¿Dudas o reporte de vulnerabilidades? Abra un issue en el repositorio o escriba al autor.
