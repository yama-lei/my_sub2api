package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayQoderModelsCatalogue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const groupID int64 = 921
	mapping := map[string]any{
		"auto":            "auto",
		"deepseek-v4-pro": "deepseek-v4-pro",
		"glm-5.3":         "glm-5.3",
		"kimi-k3":         "kimi-k3",
		"minimax-m2.7":    "minimax-m2.7",
		"custom-alias":    "private-upstream-model",
	}
	for _, tc := range []struct {
		name      string
		accounts  []service.Account
		allowlist service.GroupModelAllowlist
		want      []string
	}{
		{
			name:     "mapped models",
			accounts: []service.Account{{ID: 18, Platform: service.PlatformQoder, Credentials: map[string]any{"model_mapping": mapping}}},
			want:     []string{"auto", "custom-alias", "deepseek-v4-pro", "glm-5.3", "kimi-k3", "minimax-m2.7"},
		},
		{name: "no schedulable accounts", want: []string{}},
		{name: "missing mapping", accounts: []service.Account{{ID: 18, Platform: service.PlatformQoder}}, want: []string{}},
		{name: "other platform excluded", accounts: []service.Account{{ID: 19, Platform: service.PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"claude-only": "claude-only"}}}}, want: []string{}},
		{
			name:      "allowlist order and unknown filtering",
			accounts:  []service.Account{{ID: 18, Platform: service.PlatformQoder, Credentials: map[string]any{"model_mapping": mapping}}},
			allowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"kimi-k3", "glm-5.3", "unknown-model"}},
			want:      []string{"kimi-k3", "glm-5.3"},
		},
		{name: "allowlist cannot invent fallback models", allowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"claude-opus-5-5", "glm-5.3"}}, want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &service.Group{ID: groupID, Platform: service.PlatformQoder, ModelAllowlist: tc.allowlist}
			h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: tc.accounts}})
			rec := requestModelForTest(h, group, "", "")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var list gatewayModelsResponseForTest
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
			require.Equal(t, "list", list.Object)
			require.NotNil(t, list.Data, "empty Qoder catalogue must be [] rather than null")
			ids := make([]string, 0, len(list.Data))
			for _, model := range list.Data {
				ids = append(ids, model.ID)
				require.Equal(t, "model", model.Object)
				require.Equal(t, service.PlatformQoder, model.OwnedBy)
				retrieved := requestModelForTest(h, group, model.ID, "")
				require.Equal(t, http.StatusOK, retrieved.Code, retrieved.Body.String())
			}
			require.Equal(t, tc.want, ids)
			require.Equal(t, http.StatusNotFound, requestModelForTest(h, group, "private-upstream-model", "").Code)

			codex := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(codex)
			c.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.147.0", nil)
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group})
			h.CodexModels(c)
			require.Equal(t, http.StatusOK, codex.Code, codex.Body.String())
			var manifest codexModelsResponseForTest
			require.NoError(t, json.Unmarshal(codex.Body.Bytes(), &manifest))
			slugs := make([]string, 0, len(manifest.Models))
			for _, model := range manifest.Models {
				slugs = append(slugs, model.Slug)
			}
			require.Equal(t, tc.want, slugs)
		})
	}
}
