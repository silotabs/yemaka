package secrets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	ProviderNone     = "none"
	ProviderEnv      = "env"
	ProviderKeychain = "keychain"
)

type Reference struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Service  string `json:"service,omitempty"`
	Account  string `json:"account,omitempty"`
}

type Status struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Service  string `json:"service,omitempty"`
	Account  string `json:"account,omitempty"`
	Ready    bool   `json:"ready"`
	Detail   string `json:"detail"`
}

func Env(name string) Reference {
	return Reference{Provider: ProviderEnv, Name: strings.TrimSpace(name)}
}

func None() Reference {
	return Reference{Provider: ProviderNone}
}

func Normalize(ref Reference) Reference {
	ref.Provider = strings.ToLower(strings.TrimSpace(ref.Provider))
	ref.Name = strings.TrimSpace(ref.Name)
	ref.Service = strings.TrimSpace(ref.Service)
	ref.Account = strings.TrimSpace(ref.Account)
	if ref.Provider == "" {
		ref.Provider = ProviderNone
	}
	if ref.Provider == ProviderKeychain {
		if ref.Service == "" {
			ref.Service = "Yemaka"
		}
		if ref.Account == "" {
			ref.Account = ref.Name
		}
	}
	return ref
}

func Validate(ref Reference, required bool) error {
	ref = Normalize(ref)
	switch ref.Provider {
	case ProviderNone:
		if required {
			return fmt.Errorf("secret reference is required")
		}
		return nil
	case ProviderEnv:
		if ref.Name == "" {
			return fmt.Errorf("env secret reference requires name")
		}
		if LooksLikeSecretValue(ref.Name) {
			return fmt.Errorf("secret reference name looks like a secret value")
		}
		return nil
	case ProviderKeychain:
		if ref.Service == "" || ref.Account == "" {
			return fmt.Errorf("keychain secret reference requires service and account")
		}
		if LooksLikeSecretValue(ref.Service) || LooksLikeSecretValue(ref.Account) {
			return fmt.Errorf("keychain secret reference looks like a secret value")
		}
		return nil
	default:
		return fmt.Errorf("unsupported secret provider %q", ref.Provider)
	}
}

func Resolve(ctx context.Context, ref Reference) (string, bool, error) {
	ref = Normalize(ref)
	if err := Validate(ref, false); err != nil {
		return "", false, err
	}
	switch ref.Provider {
	case ProviderNone:
		return "", true, nil
	case ProviderEnv:
		value := strings.TrimSpace(os.Getenv(ref.Name))
		return value, value != "", nil
	case ProviderKeychain:
		value, err := resolveKeychain(ctx, ref)
		return value, strings.TrimSpace(value) != "", err
	default:
		return "", false, fmt.Errorf("unsupported secret provider %q", ref.Provider)
	}
}

func Check(ctx context.Context, ref Reference, required bool) Status {
	ref = Normalize(ref)
	status := Status{
		Provider: ref.Provider,
		Name:     ref.Name,
		Service:  ref.Service,
		Account:  ref.Account,
	}
	if err := Validate(ref, required); err != nil {
		status.Detail = err.Error()
		return status
	}
	if ref.Provider == ProviderNone {
		status.Ready = !required
		status.Detail = "no secret required"
		return status
	}
	_, ready, err := Resolve(ctx, ref)
	status.Ready = ready
	if err != nil {
		status.Detail = err.Error()
	} else if ready {
		status.Detail = "secret reference resolves"
	} else {
		status.Detail = "secret reference is not set"
	}
	return status
}

func LooksLikeSecretValue(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "sk-") ||
		strings.HasPrefix(lower, "xox") ||
		strings.HasPrefix(lower, "ghp_") ||
		strings.HasPrefix(lower, "pat_") ||
		strings.Contains(value, " ")
}

func resolveKeychain(ctx context.Context, ref Reference) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("keychain secrets are only available on macOS")
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(checkCtx, "security", "find-generic-password", "-s", ref.Service, "-a", ref.Account, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain secret is not available")
	}
	return strings.TrimSpace(string(output)), nil
}
