package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunConnectorRegistryAndTokenEnvGuard(t *testing.T) {
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runConnector(context.Background(), app, []string{"registry"}, strings.NewReader(""), &out); err != nil {
		t.Fatalf("runConnector(registry) error = %v", err)
	}
	if !strings.Contains(out.String(), `"manifest"`) || !strings.Contains(out.String(), `"provider": "env"`) {
		t.Fatalf("registry output = %q, want manifest and env secret refs", out.String())
	}

	out.Reset()
	err := runConnector(context.Background(), app, []string{"enable", "local_api", "--token-env", "sk-not-an-env-name"}, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "environment variable name") {
		t.Fatalf("enable with secret-looking token env error = %v, want guard", err)
	}
}
