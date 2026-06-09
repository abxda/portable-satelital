// satlab-keygen genera el par de llaves Ed25519 del proyecto.
//
//	go run ./cmd/satlab-keygen <dir-destino>
//
// Escribe:
//	<dir>/satlab_ed25519.key  (SEED privado, base64 — NUNCA commitear)
//	<dir>/satlab_ed25519.pub  (llave pública, base64 — se embebe en el launcher)
//
// Se niega a sobrescribir una llave existente (rotarla = decisión manual).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: satlab-keygen <dir-destino>")
		os.Exit(2)
	}
	dir := os.Args[1]
	keyPath := filepath.Join(dir, "satlab_ed25519.key")
	pubPath := filepath.Join(dir, "satlab_ed25519.pub")
	if _, err := os.Stat(keyPath); err == nil {
		fmt.Fprintf(os.Stderr, "YA EXISTE %s — no sobrescribo llaves. Bórrala a mano si de verdad quieres rotar.\n", keyPath)
		os.Exit(1)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	// Guardamos el SEED (32 bytes), no la llave expandida: es la forma canónica.
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(priv.Seed())+"\n"), 0o600); err != nil {
		panic(err)
	}
	if err := os.WriteFile(pubPath, []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0o644); err != nil {
		panic(err)
	}
	fmt.Println("llave privada:", keyPath, "(NUNCA commitear; ya está en .gitignore)")
	fmt.Println("llave pública:", pubPath)
	fmt.Println("pubkey base64:", base64.StdEncoding.EncodeToString(pub))
}
