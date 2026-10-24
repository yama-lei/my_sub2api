package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type relayQuotaRouteRepo struct {
	service.AccountRepository
	account *service.Account
}

func (r relayQuotaRouteRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}

func TestRelayQuotaRoutesRequireAccountOwnershipBeforeUpstreamHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	owner := int64(7)
	for _, route := range []struct {
		method, path     string
		downstreamStatus int
	}{
		{"GET", "/accounts/8/claude/reset-credits", http.StatusServiceUnavailable},
		{"POST", "/accounts/8/claude/reset-credits/redeem", http.StatusServiceUnavailable},
		{"GET", "/openai/accounts/8/quota", http.StatusBadRequest},
		{"POST", "/openai/accounts/8/quota/refresh", http.StatusBadRequest},
		{"POST", "/openai/accounts/8/reset-quota", http.StatusBadRequest},
	} {
		for _, caller := range []int64{0, 7, 8} {
			t.Run(fmt.Sprintf("%s%s/user-%d", route.method, route.path, caller), func(t *testing.T) {
				repo := relayQuotaRouteRepo{account: &service.Account{ID: 8, OwnerUserID: &owner}}
				h := &handler.Handlers{Admin: &handler.AdminHandlers{Account: &admin.AccountHandler{}, OpenAIOAuth: &admin.OpenAIOAuthHandler{}},
					Relay: handler.NewRelayHandler(service.NewRelayService(nil, repo, nil, nil, nil, nil, nil), nil)}
				router := gin.New()
				if caller > 0 {
					router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: caller}) })
				}
				registerRelayAccountRoutes(router.Group("/relay"), h)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, httptest.NewRequest(route.method, "/relay"+route.path, nil))
				want := route.downstreamStatus // Owner reaches the intentionally unconfigured upstream handler.
				if caller == 0 {
					want = http.StatusUnauthorized
				} else if caller != owner {
					want = http.StatusNotFound
				}
				require.Equal(t, want, recorder.Code, recorder.Body.String())
			})
		}
	}
}
