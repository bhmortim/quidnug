//go:build !pkcs11

package hsm

import "github.com/quidnug/quidnug/pkg/signer"

// Open returns an explanatory error when the binary was built without
// the pkcs11 build tag. This keeps the main module buildable without a
// PKCS#11 toolchain while making the capability opt-in.
//
// To enable the real implementation:
//
//	go build -tags=pkcs11 ./...
//
// You must have:
//   - A PKCS#11 shared library installed on the host.
//   - CGo available (CGO_ENABLED=1) and a C toolchain.
func Open(_ Config) (signer.Signer, error) {
	return nil, errf(
		"pkcs11 support not compiled in: rebuild with `-tags=pkcs11` " +
			"(requires CGo + a PKCS#11 shared library on the host)",
	)
}
