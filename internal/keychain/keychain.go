// Package keychain does read-only lookups of existing macOS Keychain
// generic-password items. It never creates, modifies, or deletes items.
package keychain

import (
	"errors"
	"fmt"

	gokeychain "github.com/keybase/go-keychain"
)

// ErrNotFound is returned when no matching Keychain item exists.
var ErrNotFound = errors.New("keychain: item not found")

// LookupPassword returns the secret for the generic-password item matching
// service and account in the user's default (login) keychain.
func LookupPassword(service, account string) (string, error) {
	data, err := gokeychain.GetGenericPassword(service, account, "", "")
	if err != nil {
		return "", fmt.Errorf("keychain lookup service=%q account=%q: %w", service, account, err)
	}
	if data == nil {
		return "", fmt.Errorf("service=%q account=%q: %w", service, account, ErrNotFound)
	}
	return string(data), nil
}
