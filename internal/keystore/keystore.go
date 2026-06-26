// Package keystore stores signing private keys in the OS keychain — macOS
// Keychain, Linux Secret Service (libsecret/gnome-keyring), or Windows
// Credential Manager — via go-keyring, so users can sign without passing a raw
// private key on every command.
//
// Only the secret material lives here; the human-facing index (name → derived
// address) is kept in the config file (public info), because the OS keyrings
// expose no portable "list everything under a service" API.
package keystore

import "github.com/zalando/go-keyring"

// service namespaces all rail0 CLI secrets within the OS keyring.
const service = "rail0-cli"

// Set stores (or replaces) the private key for a named account.
func Set(name, privateKeyHex string) error {
	return keyring.Set(service, name, privateKeyHex)
}

// Get returns the stored private key for a named account.
func Get(name string) (string, error) {
	return keyring.Get(service, name)
}

// Delete removes a named account's key. Deleting a missing key is not an error.
func Delete(name string) error {
	err := keyring.Delete(service, name)
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}
