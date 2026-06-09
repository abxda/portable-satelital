// Package sign verifica la firma Ed25519 del catálogo (manifest) y de los
// descriptores de auto-actualización. La llave PÚBLICA viaja embebida en el
// binario: aunque alguien comprometiera el hosting (Hugging Face), un
// manifiesto alterado NO verificaría y el launcher se niega a descargar.
//
// La llave privada vive FUERA del repo (credentials/, gitignored) y solo se
// usa al publicar (cmd/satlab-sign).
package sign

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// PubKeyB64 es la llave pública Ed25519 del proyecto (32 bytes, base64).
// Generada con cmd/satlab-keygen. Si se rota la llave hay que re-publicar el
// launcher (a propósito: la confianza ancla en el binario, no en el hosting).
const PubKeyB64 = "hv3mFtFlZbcFhFRTQCn8xyEOM9fgITMf5yHsfmXYG9I="

func pubKey() (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(PubKeyB64)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("llave pública embebida inválida")
	}
	return ed25519.PublicKey(b), nil
}

// Verify comprueba que sigB64 (base64, posiblemente con espacios/saltos) sea
// la firma Ed25519 de data bajo la llave pública embebida.
func Verify(data, sigB64 []byte) error {
	pk, err := pubKey()
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("firma malformada")
	}
	if !ed25519.Verify(pk, data, sig) {
		return fmt.Errorf("la firma Ed25519 NO verifica: el catálogo fue alterado o no proviene del autor")
	}
	return nil
}

// Fingerprint devuelve una huella corta y legible de la llave pública
// (primeros 8 bytes del SHA-256), para mostrarla en la bitácora de la UI y en
// VERIFICACION.md.
func Fingerprint() string {
	b, err := base64.StdEncoding.DecodeString(PubKeyB64)
	if err != nil {
		return "?"
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}
