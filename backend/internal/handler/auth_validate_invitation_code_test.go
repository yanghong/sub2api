//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type validateInvitationSettingRepoStub struct {
	values map[string]string
}

func (s *validateInvitationSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *validateInvitationSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *validateInvitationSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *validateInvitationSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (s *validateInvitationSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *validateInvitationSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *validateInvitationSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type validateInvitationRedeemRepoStub struct{}

func (s *validateInvitationRedeemRepoStub) Create(context.Context, *service.RedeemCode) error {
	panic("unexpected Create call")
}

func (s *validateInvitationRedeemRepoStub) CreateBatch(context.Context, []service.RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (s *validateInvitationRedeemRepoStub) GetByID(context.Context, int64) (*service.RedeemCode, error) {
	panic("unexpected GetByID call")
}

func (s *validateInvitationRedeemRepoStub) GetByCode(_ context.Context, code string) (*service.RedeemCode, error) {
	if code != "invite-ok" {
		return nil, service.ErrRedeemCodeNotFound
	}
	return &service.RedeemCode{
		ID:     9001,
		Code:   "invite-ok",
		Type:   service.RedeemTypeInvitation,
		Status: service.StatusUnused,
	}, nil
}

func (s *validateInvitationRedeemRepoStub) Update(context.Context, *service.RedeemCode) error {
	panic("unexpected Update call")
}

func (s *validateInvitationRedeemRepoStub) BatchUpdate(context.Context, []int64, service.RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (s *validateInvitationRedeemRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *validateInvitationRedeemRepoStub) Use(context.Context, int64, int64) error {
	panic("unexpected Use call")
}

func (s *validateInvitationRedeemRepoStub) List(context.Context, pagination.PaginationParams) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *validateInvitationRedeemRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *validateInvitationRedeemRepoStub) ListByUser(context.Context, int64, int) ([]service.RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (s *validateInvitationRedeemRepoStub) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (s *validateInvitationRedeemRepoStub) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func TestValidateInvitationCodeWorksWhenInvitationSettingDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	settingSvc := service.NewSettingService(&validateInvitationSettingRepoStub{
		values: map[string]string{
			service.SettingKeyInvitationCodeEnabled: "false",
		},
	}, cfg)
	redeemSvc := service.NewRedeemService(&validateInvitationRedeemRepoStub{}, nil, nil, nil, nil, nil, nil, nil)
	handler := NewAuthHandler(cfg, nil, nil, settingSvc, nil, redeemSvc, nil, nil)

	router := gin.New()
	router.POST("/api/v1/auth/validate-invitation-code", handler.ValidateInvitationCode)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/validate-invitation-code", bytes.NewBufferString(`{"code":"invite-ok"}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeJSONBody(t, recorder)
	data := payload["data"].(map[string]any)
	require.Equal(t, true, data["valid"])
}
