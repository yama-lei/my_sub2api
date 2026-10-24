package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type qoderRelayRouteRepo struct {
	service.AccountRepository
	account *service.Account
}

func (r qoderRelayRouteRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}

func TestQoderRelayRoutesEnforceAuthenticationOwnershipAndProxyPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	foreignOwner := int64(2)
	repo := qoderRelayRouteRepo{account: &service.Account{ID: 8, Platform: service.PlatformQoder,
		Type: service.AccountTypeOAuth, OwnerUserID: &foreignOwner}}
	qoder := service.NewQoderService(repo, nil, nil)
	t.Cleanup(qoder.Stop)
	h := &handler.Handlers{Admin: &handler.AdminHandlers{QoderOAuth: admin.NewQoderOAuthHandler(qoder, nil)},
		Relay: handler.NewRelayHandler(service.NewRelayService(nil, repo, nil, nil, nil, nil, nil), nil)}
	for _, test := range []struct {
		name               string
		user               int64
		method, path, body string
		status             int
	}{
		{name: "anonymous", method: "POST", path: "/qoder/oauth/start", body: `{"name":"Qoder"}`, status: http.StatusUnauthorized},
		{name: "foreign reauthorization", user: 1, method: "POST", path: "/qoder/oauth/start", body: `{"account_id":8}`, status: http.StatusNotFound},
		{name: "foreign credits", user: 1, method: "GET", path: "/qoder/credits/accounts/8", status: http.StatusNotFound},
		{name: "platform proxy", user: 1, method: "POST", path: "/qoder/oauth/start", body: `{"name":"Qoder","proxy_id":5}`, status: http.StatusBadRequest},
		{name: "native user authorization", user: 1, method: "POST", path: "/qoder/oauth/start", body: `{"name":"Qoder","group_ids":[9]}`, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			if test.user > 0 {
				router.Use(func(c *gin.Context) {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: test.user})
				})
			}
			registerRelayAccountRoutes(router.Group("/relay"), h)
			request := httptest.NewRequest(test.method, "/relay"+test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, test.status, recorder.Code, recorder.Body.String())
			if test.status == http.StatusOK {
				require.Contains(t, recorder.Body.String(), "qoder.com.cn/device/selectAccounts")
				require.NotContains(t, recorder.Body.String(), "access_token")
			}
		})
	}
}
