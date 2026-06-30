//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthService_LoginOrRegisterOAuth_RequiresInvitationForNewUserWhenSettingDisabled(t *testing.T) {
	repo := &userRepoStub{}
	service := newAuthServiceWithRedeemRepo(repo, map[string]string{
		SettingKeyRegistrationEnabled:   "true",
		SettingKeyInvitationCodeEnabled: "false",
	}, nil, nil, &redeemCodeRepoStub{})
	service.refreshTokenCache = &refreshTokenCacheStub{}

	_, _, err := service.LoginOrRegisterOAuthWithTokenPair(context.Background(), "oauth@example.com", "OAuth User", "", "", "github")
	require.ErrorIs(t, err, ErrOAuthInvitationRequired)
	require.Empty(t, repo.created)
}

func TestAuthService_LoginOrRegisterOAuth_ExistingUserDoesNotRequireInvitation(t *testing.T) {
	existing := &User{ID: 9, Email: "oauth@example.com", Role: RoleUser, Status: StatusActive}
	repo := &userRepoStub{user: existing}
	service := newAuthServiceWithRedeemRepo(repo, map[string]string{
		SettingKeyRegistrationEnabled:   "true",
		SettingKeyInvitationCodeEnabled: "false",
	}, nil, nil, &redeemCodeRepoStub{})
	service.refreshTokenCache = &refreshTokenCacheStub{}

	tokenPair, user, err := service.LoginOrRegisterOAuthWithTokenPair(context.Background(), "oauth@example.com", "OAuth User", "", "", "github")
	require.NoError(t, err)
	require.NotNil(t, tokenPair)
	require.Equal(t, existing.ID, user.ID)
}
