package heartbeat

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestHeartbeatLoopTickRecordsReport(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	loop := Loop{
		Enabled: true,
		Store:   store,
		Reporter: ReporterFunc(func(ctx context.Context) (Report, error) {
			return Report{
				Overall:     StatusOK,
				GeneratedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				Checks: []Check{{
					ID:        "hb_test",
					Name:      "sqlite",
					Status:    StatusOK,
					Detail:    "ok",
					CheckedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				}},
			}, nil
		}),
	}
	report, err := loop.Tick(ctx)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if report.Overall != StatusOK {
		t.Fatalf("Overall = %q, want ok", report.Overall)
	}
	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if len(latest.Checks) != 1 || latest.Checks[0].Name != "sqlite" {
		t.Fatalf("latest = %+v, want recorded sqlite check", latest)
	}
}

func TestHeartbeatLoopDisabled(t *testing.T) {
	_, err := (&Loop{}).Tick(context.Background())
	if err == nil {
		t.Fatal("Tick() error = nil, want disabled error")
	}
}
