// Package azure wraps the az CLI for interacting with Azure Key Vault.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
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

// azTimeout bounds every az invocation. Without it a stalled network call,
// a token refresh waiting on an interactive login, or a hung Python process
// would leave the UI stuck in a "⏳ Loading…" state forever.
const azTimeout = 60 * time.Second

// runAZ executes an az CLI command and returns its stdout on success.
// On failure, stderr is included in the error message.
//
// Hang protection:
//   - the whole call is bounded by azTimeout;
//   - stdin is /dev/null, so interactive prompts get EOF instead of waiting;
//   - az runs in its own process group. Once az exits, any detached children
//     it left behind (which would otherwise hold the output pipes open and
//     block reading forever) are killed together with the group.
func runAZ(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), azTimeout)
	defer cancel()

	cmd := exec.Command("az", args...)
	cmd.Stdin = nil // /dev/null
	cmd.Env = append(os.Environ(),
		"AZURE_CORE_COLLECT_TELEMETRY=no",
		"AZURE_CORE_NO_COLOR=true",
		"AZURE_CORE_ONLY_SHOW_ERRORS=true",
		"AZURE_EXTENSION_USE_DYNAMIC_INSTALL=no",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Our own pipes (instead of cmd.Output) so that cmd.Wait returns as soon
	// as az exits, independent of who else still holds the write ends.
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe: %w", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return nil, fmt.Errorf("creating pipe: %w", err)
	}
	defer outR.Close()
	defer errR.Close()
	cmd.Stdout, cmd.Stderr = outW, errW

	startErr := cmd.Start()
	// The child has its own copies; close ours so EOF arrives when it's done.
	outW.Close()
	errW.Close()
	if startErr != nil {
		return nil, fmt.Errorf("exec az: %w", startErr)
	}

	var stdout, stderr bytes.Buffer
	readDone := make(chan struct{}, 2)
	go func() { io.Copy(&stdout, outR); readDone <- struct{}{} }()
	go func() { io.Copy(&stderr, errR); readDone <- struct{}{} }()

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	killGroup := func() { syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }

	var runErr error
	select {
	case runErr = <-waitDone:
	case <-ctx.Done():
		killGroup()
		<-waitDone
		return nil, fmt.Errorf("az %s: timed out after %s (check network / run 'az login')",
			strings.Join(args[:min(3, len(args))], " "), azTimeout)
	}

	// az has exited. Its output is already in the pipe buffers; give the
	// readers a moment to drain, then kill any leftover children so the
	// pipes close and the readers return.
	drain := time.NewTimer(500 * time.Millisecond)
	defer drain.Stop()
	for n := 0; n < 2; n++ {
		select {
		case <-readDone:
		case <-drain.C:
			killGroup()
			outR.Close()
			errR.Close()
			<-readDone
		}
	}
	killGroup() // reap any stragglers even if pipes closed normally

	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); ok {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = runErr.Error()
			}
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("exec az: %w", runErr)
	}
	return stdout.Bytes(), nil
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
//
// A single az call returns both fields, so the value and version always
// belong to the same revision (two separate calls could straddle a
// concurrent update) and the read takes half as long.
func GetSecret(vault, name string) (value, version string, err error) {
	out, err := runAZ("keyvault", "secret", "show",
		"--vault-name", vault,
		"--name", name,
		"--query", "{id:id, value:value}",
		"-o", "json")
	if err != nil {
		return "", "", fmt.Errorf("reading secret: %w", err)
	}
	var res struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return "", "", fmt.Errorf("parsing secret: %w", err)
	}
	return res.Value, versionFromID(res.ID), nil
}

// versionFromID extracts the trailing version segment of a Key Vault secret ID.
func versionFromID(id string) string {
	id = strings.TrimSpace(id)
	return id[strings.LastIndex(id, "/")+1:]
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
	return versionFromID(string(out)), nil
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
func SetSecret(vault, name, value, expectedVersion string) (newVersion string, err error) {
	if expectedVersion != "" {
		liveVersion, err := GetCurrentVersion(vault, name)
		if err == nil && liveVersion != "" && liveVersion != expectedVersion {
			return "", fmt.Errorf(
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
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	tmp.Close()

	out, err := runAZ("keyvault", "secret", "set",
		"--vault-name", vault,
		"--name", name,
		"--file", tmp.Name(),
		"--query", "id",
		"-o", "tsv")
	if err != nil {
		return "", fmt.Errorf("az keyvault secret set: %w", err)
	}
	return versionFromID(string(out)), nil
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
		ver := versionFromID(r.ID)
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
		"-o", "json")
	if err != nil {
		return "", fmt.Errorf("az keyvault secret show --version: %w", err)
	}
	var value string
	if err := json.Unmarshal(out, &value); err != nil {
		return "", fmt.Errorf("parsing secret value: %w", err)
	}
	return value, nil
}
