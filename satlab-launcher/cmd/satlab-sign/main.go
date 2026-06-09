// satlab-sign firma un archivo (manifest.txt, descriptor de self-update) con
// la llave privada Ed25519 del proyecto y escribe <archivo>.sig (base64).
//
//	go run ./cmd/satlab-sign <llave-privada.key> <archivo> [archivo...]
//
// La verificación simétrica vive en internal/sign (launcher) — misma
// convención: firma sobre los BYTES EXACTOS del archivo, firma en base64.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "uso: satlab-sign <llave-privada.key> <archivo> [archivo...]")
		os.Exit(2)
	}
	seedB64, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(seedB64)))
	if err != nil || len(seed) != ed25519.SeedSize {
		fmt.Fprintln(os.Stderr, "llave privada inválida")
		os.Exit(1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	for _, path := range os.Args[2:] {
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		sig := ed25519.Sign(priv, data)
		out := path + ".sig"
		if err := os.WriteFile(out, []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644); err != nil {
			panic(err)
		}
		fmt.Println("firmado:", out)
	}
}
