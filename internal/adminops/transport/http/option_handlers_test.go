package http

import (
	stdhttp "net/http"
	"strings"
	"testing"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
)

func TestGetOptionsHidesSensitiveKeys(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	originalOptions := platformconfig.OptionMap
	platformconfig.OptionMap = map[string]string{
		"SystemName":      "codego-api",
		"SMTPToken":       "secret-token",
		"ModelRatio":      `{"gpt-4o":15}`,
		"CompletionRatio": `{"gpt-4o":2}`,
	}
	t.Cleanup(func() {
		platformconfig.OptionMap = originalOptions
	})

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/option/", nil)
	GetOptions(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected get options to succeed, got %#v", response)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "secret-token") {
		t.Fatalf("expected sensitive token to be hidden, got body %s", body)
	}
	if !strings.Contains(body, "CompletionRatioMeta") {
		t.Fatalf("expected completion ratio meta in response, got body %s", body)
	}
}

func TestUpdateOptionRejectsComplianceField(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPut, "/api/option/", map[string]any{
		"key":   "payment_setting.compliance_confirmed",
		"value": true,
	})
	UpdateOption(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected compliance field update to fail")
	}
}

func TestMigrateConsoleSettingMovesLegacyOptions(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)

	seed := []platformschema.Option{
		{Key: "Announcements", Value: `[{"title":"x"}]`},
		{Key: "UptimeKumaUrl", Value: "https://status.example.com"},
		{Key: "UptimeKumaSlug", Value: "prod"},
	}
	for i := range seed {
		if err := platformdb.DB.Create(&seed[i]).Error; err != nil {
			t.Fatalf("failed to seed option: %v", err)
		}
	}

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/option/migrate_console_setting", nil)
	MigrateConsoleSetting(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected console migration to succeed, got %#v", response)
	}

	var announcement platformschema.Option
	if err := platformdb.DB.Where("key = ?", "console_setting.announcements").First(&announcement).Error; err != nil {
		t.Fatalf("expected migrated announcements option, got error %v", err)
	}
	if announcement.Value == "" {
		t.Fatalf("expected migrated announcements value to be non-empty")
	}

	var oldAnnouncement platformschema.Option
	if err := platformdb.DB.Where("key = ?", "Announcements").First(&oldAnnouncement).Error; err == nil {
		t.Fatalf("expected old announcement option to be deleted")
	}
}
