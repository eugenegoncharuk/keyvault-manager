// Package azure wraps the az CLI for interacting with Azure Key Vault.
package azure

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Subscription represents an Azure subscription returned by az account list.
type Subscription struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

// SecretVersion represents a single version of a Key Vault secret.
type SecretVersion struct {
	ID      string
	Version string
	Updated time.Time
	Enabled bool
}

// runAZ executes an az CLI command and returns its combined stdout on success.
// On failure, stderr is included in the error message.
func runAZ(args ...string) ([]byte, error) {
	cmd := exec.Command("az", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("exec az: %w", err)
	}
	return out, nil
}

// SetSubscription sets the active Azure subscription for subsequent az calls.
func SetSubscription(id string) error {
	_, err := runAZ("account", "set", "--subscription", id)
	if err != nil {
		return fmt.Errorf("az account set: %w", err)
	}
	return nil
}

// ListVaults returns the names of all Key Vaults in the current subscription.
func ListVaults() ([]string, error) {
	out, err := runAZ("keyvault", "list", "--query", "[].name", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("az keyvault list: %w", err)
	}
	var vaults []string
	if err := json.Unmarshal(out, &vaults); err != nil {
		return nil, fmt.Errorf("parsing vaults: %w", err)
	}
	return vaults, nil
}

// GetSecret retrieves the current value and version ID of the named secret.
func GetSecret(vault, name string) (value, version string, err error) {
	// Fetch ID (which encodes the version at the end)
	out, err := runAZ("keyvault", "secret", "show",
		"--vault-name", vault,
		"--name", name,
		"--query", "id",
		"-o", "tsv")
	if err != nil {
		return "", "", fmt.Errorf("reading secret id: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "/")
	version = parts[len(parts)-1]

	// Fetch value
	out, err = runAZ("keyvault", "secret", "show",
		"--vault-name", vault,
		"--name", name,
		"--query", "value",
		"-o", "tsv")
	if err != nil {
		return "", "", fmt.Errorf("reading secret value: %w", err)
	}
	return strings.TrimSpace(string(out)), version, nil
}

// GetCurrentVersion returns only the live version ID of the named secret.
func GetCurrentVersion(vault, name string) (string, error) {
	out, err := runAZ("keyvault", "secret", "show",
		"--vault-name", vault,
		"--name", name,
		"--query", "id",
		"-o", "tsv")
	if err != nil {
		return "", fmt.Errorf("reading version: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "/")
	return parts[len(parts)-1], nil
}

// SetSecret uploads a new secret value to Azure Key Vault.
//
// If expectedVersion is non-empty, the current live version is checked first.
// A mismatch means someone else updated the secret — an error is returned to
// prevent accidentally overwriting concurrent changes (same semantics as
// push-secret.sh).
//
// The value is written to a temporary file so that there is no command-line
// length limit on large YAML/JSON secrets.
func SetSecret(vault, name, value, expectedVersion string) error {
	if expectedVersion != "" {
		liveVersion, err := GetCurrentVersion(vault, name)
		if err == nil && liveVersion != "" && liveVersion != expectedVersion {
			return fmt.Errorf(
				"version conflict — secret was updated externally.\n\n"+
					"Live version:     %s\n"+
					"Expected version: %s\n\n"+
					"Please re-read the secret before pushing.",
				liveVersion, expectedVersion)
		}
	}

	// Write to a temp file to avoid arg-length limits for large secrets.
	tmp, err := os.CreateTemp("", "kv-secret-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmp.Close()

	_, err = runAZ("keyvault", "secret", "set",
		"--vault-name", vault,
		"--name", name,
		"--file", tmp.Name())
	if err != nil {
		return fmt.Errorf("az keyvault secret set: %w", err)
	}
	return nil
}

// ListSecretVersions returns all versions of the named secret, newest first.
func ListSecretVersions(vault, name string) ([]SecretVersion, error) {
	type rawVer struct {
		ID         string `json:"id"`
		Attributes struct {
			Updated string `json:"updated"`
			Enabled bool   `json:"enabled"`
		} `json:"attributes"`
	}

	out, err := runAZ("keyvault", "secret", "list-versions",
		"--vault-name", vault,
		"--name", name,
		"-o", "json")
	if err != nil {
		return nil, fmt.Errorf("az keyvault secret list-versions: %w", err)
	}

	var raw []rawVer
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing versions: %w", err)
	}

	result := make([]SecretVersion, 0, len(raw))
	for _, r := range raw {
		parts := strings.Split(r.ID, "/")
		ver := parts[len(parts)-1]
		t, _ := time.Parse(time.RFC3339, r.Attributes.Updated)
		result = append(result, SecretVersion{
			ID:      r.ID,
			Version: ver,
			Updated: t,
			Enabled: r.Attributes.Enabled,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Updated.After(result[j].Updated)
	})
	return result, nil
}

// GetSecretByVersion retrieves the value of a specific, historical secret version.
func GetSecretByVersion(vault, name, version string) (string, error) {
	out, err := runAZ("keyvault", "secret", "show",
		"--vault-name", vault,
		"--name", name,
		"--version", version,
		"--query", "value",
		"-o", "tsv")
	if err != nil {
		return "", fmt.Errorf("az keyvault secret show --version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
