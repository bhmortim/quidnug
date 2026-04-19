package client

import "errors"

// errorsAsStd exists so the inline errorsAs helper in client.go can
// fall back to the stdlib without importing "errors" in that file.
// (client.go already imports the stdlib errors indirectly; the split
// just keeps the import list in client.go focused.)
func errorsAsStd(err error, target any) bool {
	return errors.As(err, target)
}
