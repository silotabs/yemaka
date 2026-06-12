package heartbeat

type Check struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
	CheckedAt string `json:"checkedAt"`
}

type Report struct {
	Overall     string  `json:"overall"`
	GeneratedAt string  `json:"generatedAt"`
	Checks      []Check `json:"checks"`
}

const (
	StatusHealthy     = "healthy"
	StatusWarning     = "warning"
	StatusBroken      = "broken"
	StatusDisabled    = "disabled"
	StatusNeedsAuth   = "needs_auth"
	StatusNeedsConfig = "needs_config"

	StatusOK    = StatusHealthy
	StatusWarn  = StatusWarning
	StatusError = StatusBroken
)
