package secrets

import (
	"context"
	"strings"
	"testing"
)

func TestEnvReferenceResolvesWithoutExposingConfigValue(t *testing.T) {
	t.Setenv("YEMAKA_TEST_SECRET_REF", "secret-value")
	value, ready, err := Resolve(context.Background(), Env("YEMAKA_TEST_SECRET_REF"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !ready || value != "secret-value" {
		t.Fatalf("Resolve() = %q, %t; want secret-value, true", value, ready)
	}
	status := Check(context.Background(), Env("YEMAKA_TEST_SECRET_REF"), true)
	if !status.Ready || status.Name != "YEMAKA_TEST_SECRET_REF" {
		t.Fatalf("status = %+v, want ready env reference", status)
	}
}

func TestRejectsSecretLookingReferenceName(t *testing.T) {
	err := Validate(Env("sk-real-secret-value"), true)
	if err == nil || !strings.Contains(err.Error(), "secret value") {
		t.Fatalf("Validate() error = %v, want secret-looking name rejection", err)
	}
}

func TestNoneReferenceIsReadyWhenOptional(t *testing.T) {
	status := Check(context.Background(), None(), false)
	if !status.Ready || status.Provider != ProviderNone {
		t.Fatalf("status = %+v, want optional none ready", status)
	}
}
