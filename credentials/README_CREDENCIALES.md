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

## GitHub — SSH + gh CLI (NO hay archivo de token)
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

---

## Disciplina del manifest (igual que en Big Data)
Edición **aditiva por clave**, **anti-clobber**: baja el manifest → cambia UNA clave → diff de 1 línea →
sube → re-baja → confirma que **COINCIDE**. Líneas en **LF**. `sha256` de 64 hex. Verifica que el `oid`
LFS del archivo subido == su sha256 antes de registrar la clave.
