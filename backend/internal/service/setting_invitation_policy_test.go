//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestInvitationPolicy_CannotBeDisabled(t *testing.T) {
	for _, value := range []string{"", "false", "true", "invalid"} {
		t.Run(value, func(t *testing.T) {
			svc := NewSettingService(&settingRepoStub{values: map[string]string{
				SettingKeyInvitationCodeEnabled: value,
			}}, &config.Config{})
			require.True(t, svc.IsInvitationCodeEnabled(context.Background()))
		})
	}
	svc := NewSettingService(&settingRepoStub{err: errors.New("settings unavailable")}, &config.Config{})
	require.True(t, svc.IsInvitationCodeEnabled(context.Background()))
}

func TestInvitationPolicy_PublicAndAdminSettingsAgree(t *testing.T) {
	values := map[string]string{SettingKeyInvitationCodeEnabled: "false"}
	svc := NewSettingService(&settingPublicRepoStub{values: values}, &config.Config{})
	public, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.InvitationCodeEnabled)
	require.True(t, svc.parseSettings(values).InvitationCodeEnabled)
}

func TestInvitationPolicy_AdminUpdateKeepsInvitationsRequired(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{InvitationCodeEnabled: false}))
	require.Equal(t, "true", repo.updates[SettingKeyInvitationCodeEnabled])
}

func TestInvitationPolicy_RejectsEmailSignupWithoutCode(t *testing.T) {
	for _, code := range []string{"", " \t "} {
		t.Run(code, func(t *testing.T) {
			repo := &userRepoStub{}
			svc := newAuthService(repo, map[string]string{
				SettingKeyRegistrationEnabled:   "true",
				SettingKeyInvitationCodeEnabled: "false",
			}, nil, nil)
			_, user, err := svc.RegisterWithVerification(context.Background(), "new@test.com", "password", "", "", code, "")
			require.ErrorIs(t, err, ErrInvitationCodeRequired)
			require.Nil(t, user)
			require.Empty(t, repo.created)
		})
	}
}

func TestInvitationPolicy_RejectsOAuthSignupWithoutCode(t *testing.T) {
	for _, code := range []string{"", " \t "} {
		t.Run(code, func(t *testing.T) {
			repo := &userRepoStub{}
			svc := newAuthService(repo, map[string]string{
				SettingKeyRegistrationEnabled:   "true",
				SettingKeyInvitationCodeEnabled: "false",
			}, nil, nil)
			svc.refreshTokenCache = &refreshTokenCacheStub{}
			_, user, err := svc.LoginOrRegisterOAuthWithTokenPair(context.Background(), "new@test.com", "new", code, "", "oidc")
			require.ErrorIs(t, err, ErrOAuthInvitationRequired)
			require.Nil(t, user)
			require.Empty(t, repo.created)
		})
	}
}

func TestInvitationPolicy_RejectsOAuthEmailSignupWithoutCode(t *testing.T) {
	svc := newAuthService(&userRepoStub{}, map[string]string{
		SettingKeyRegistrationEnabled:   "true",
		SettingKeyInvitationCodeEnabled: "false",
	}, nil, nil)
	for _, code := range []string{"", " \t "} {
		_, err := svc.validateOAuthRegistrationInvitation(context.Background(), code)
		require.ErrorIs(t, err, ErrInvitationCodeRequired)
	}
}

func TestInvitationPolicy_LegacyOAuthCannotCreateAnUninvitedUser(t *testing.T) {
	repo := &userRepoStub{}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)
	_, user, err := svc.LoginOrRegisterOAuth(context.Background(), "new@test.com", "new")
	require.ErrorIs(t, err, ErrOAuthInvitationRequired)
	require.Nil(t, user)
	require.Empty(t, repo.created)
}

func TestInvitationPolicy_ExistingOAuthUserDoesNotNeedInvitation(t *testing.T) {
	repo := &userRepoStub{user: &User{ID: 7, Email: "existing@test.com", Username: "existing", Status: StatusActive}}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:   "true",
		SettingKeyInvitationCodeEnabled: "false",
	}, nil, nil)
	svc.refreshTokenCache = &refreshTokenCacheStub{}
	pair, user, err := svc.LoginOrRegisterOAuthWithTokenPair(context.Background(), "existing@test.com", "existing", "", "", "oidc")
	require.NoError(t, err)
	require.NotNil(t, pair)
	require.Equal(t, int64(7), user.ID)
	require.Empty(t, repo.created)
}

func TestInvitationPolicy_LegacyRegisterCannotCreateAnUninvitedUser(t *testing.T) {
	repo := &userRepoStub{}
	svc := newAuthService(repo, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
	_, user, err := svc.Register(context.Background(), "new@test.com", "password")
	require.ErrorIs(t, err, ErrInvitationCodeRequired)
	require.Nil(t, user)
	require.Empty(t, repo.created)
}

// Positive signup fixtures supply a real, unused invitation so their assertions
// continue to exercise quotas, identity writes and verification after the gate.
const testSignupInvitation = "test-signup-invitation"

func newTestSignupInvitationRepo() *redeemCodeRepoStub {
	return &redeemCodeRepoStub{codesByCode: map[string]*RedeemCode{
		testSignupInvitation: {ID: 1701, Code: testSignupInvitation, Type: RedeemTypeInvitation, Status: StatusUnused},
	}}
}

func registerWithTestInvitation(svc *AuthService, ctx context.Context, email, password string) (string, *User, error) {
	return svc.RegisterWithVerification(ctx, email, password, "", "", testSignupInvitation, "")
}
