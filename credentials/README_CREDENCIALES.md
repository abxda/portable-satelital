# Credenciales — uso y seguridad

## 🔒 REGLA #1 (no negociable)
**NUNCA** imprimas, hagas `echo`, ni **commitees** estos archivos. El `.gitignore` de la raíz ya los
excluye — no lo quites. Si alguno se filtra a un commit, considéralo comprometido y rótalo.

---

## Hugging Face — `.hf_token` (copiado aquí)
Token de **escritura** de la cuenta `abxda`. Úsalo **sin imprimirlo**:

```bash
export HF_TOKEN=$(cat credentials/.hf_token)

# Crear el dataset NUEVO (lo haces TÚ, dentro de la cuenta abxda):
hf repo create abxda/portable-satelital --repo-type dataset      # o se auto-crea al primer upload

# Subir artefactos:
hf upload abxda/portable-satelital <archivo_local> <ruta_en_repo> --repo-type dataset
```
Sirve para: crear el dataset, subir los tarballs (LFS), el `manifest.txt` y los launchers + descriptores.

---

## GitHub — en ESTA máquina (Windows/líder): SSH + gh CLI (NO hay archivo de token)
- **Llave SSH `~/.ssh/id_ed25519` — LIBRE y ya registrada en GitHub (cuenta `abxda`).** Es para `git push`.
  - Remote: `git@github.com:abxda/<repo>.git`
  - Push:
    ```bash
    GIT_SSH_COMMAND='ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15' git push
    ```
- **`gh` CLI ya logueado** (cuenta `abxda`, scope `repo`) — para **CREAR el repo**:
  ```bash
  gh repo create abxda/portable-satelital --public
  ```
- En **otra** máquina: `gh auth login` (o un PAT con scope `repo`) + registra la llave SSH pública.

**Resumen de la duda del Dr.:** sí, "el git" sirve para crear el proyecto nuevo en GitHub — vía
`gh repo create` (gh tiene scope `repo`); el `git push` va por **SSH**, que está libre.

### ⭐ Los agentes Linux y macOS SÍ tienen su propio `.github_token` (+ `.hf_token`)
A diferencia de esta máquina (Windows/líder, sin archivo de token → keyring + SSH), **los agentes remotos
de Linux y macOS YA tienen en SUS máquinas un archivo `.github_token` y `.hf_token`** (se usaron en el
proyecto Big Data para publicar: push/releases a los repos y subidas a HF). Su `.github_token` tiene alcance
**`repo`** (control total de repos: crear, push, releases) — justo lo necesario. Por lo tanto esos agentes
**pueden PUBLICAR DIRECTO** (crear el repo, `git push`, `hf upload`), no solo "reportar el sha".
- Cada agente confirma el suyo con `gh auth status` o probándolo (los tokens viven en sus máquinas; no se
  leen desde aquí).
- **Misma regla de seguridad:** nunca imprimir / `echo` / commitear ese token.
- **Asimetría a recordar:** `.hf_token` es un **archivo** (viaja si mueves la carpeta); el GitHub de ESTA
  máquina está **atado al equipo** (keyring + SSH, no viaja). Los agentes Linux/Mac tienen el suyo propio
  en su equipo.

---

## Disciplina del manifest (igual que en Big Data)
Edición **aditiva por clave**, **anti-clobber**: baja el manifest → cambia UNA clave → diff de 1 línea →
sube → re-baja → confirma que **COINCIDE**. Líneas en **LF**. `sha256` de 64 hex. Verifica que el `oid`
LFS del archivo subido == su sha256 antes de registrar la clave.
