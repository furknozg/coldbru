package boot

import (
	"testing"

	"github.com/furknozg/coldbru/internal/platform/app"
)

func TestSetupNeedsDatabaseAndNATS(t *testing.T) {
	if err := Setup(t.Context(), &app.Deps{}); err == nil {
		t.Error("Setup() without a database and NATS: error = nil")
	}
}
