# Registration Invitation Required Design

## Goal

Require every new user registration to provide a valid invitation code. Existing users can continue logging in normally. The requirement applies to email/password signup and all OAuth/SSO paths that create a new local user.

## Current State

Sub2API already has invitation-code support based on the public setting `invitation_code_enabled`.

- Email registration calls `AuthService.RegisterWithVerification`, which requires and consumes an invitation code only when `SettingService.IsInvitationCodeEnabled` returns true.
- OAuth first-login registration calls `AuthService.loginOrRegisterOAuthWithTokenPair`, which also requires an invitation code only when the same setting is enabled.
- OAuth email-completion flows have invitation-code helper logic, but still follow the same feature flag.
- The Vue registration form only shows the invitation-code field when public settings report `invitation_code_enabled: true`.

Because the default for `IsInvitationCodeEnabled` is false, deployments can still allow new users to register without an invitation code.

## Chosen Approach

Use backend enforcement as the source of truth and update the frontend to match.

The invitation feature flag may remain in settings for compatibility and existing admin UI, but it should no longer decide whether a new account requires an invitation code. New account creation should always require a valid, unused redeem code of type `invitation`.

This avoids relying on production settings being present or correctly enabled, and it prevents direct API callers from bypassing the frontend.

## Registration Scope

The requirement applies when a new user record is created:

- `POST /api/v1/auth/register` email/password signup.
- OAuth first login that creates a user through `LoginOrRegisterOAuthWithTokenPair` or `LoginOrRegisterOAuthWithTokenPairAndPromoCode`.
- OAuth pending/email completion flows that create a local account from a third-party identity.

The requirement does not apply to:

- Existing user login.
- OAuth login for an existing local account.
- Account binding for an already authenticated user.
- Admin-created users or imports unless they currently route through the same public signup services.

## Backend Design

`AuthService` remains the final enforcement layer.

Add or adjust a small helper that validates invitation codes for signup:

- Trim the supplied invitation code.
- If empty, return `ErrInvitationCodeRequired` for email-style registration or the existing OAuth-specific error where callers already expect it.
- Look up the redeem code by code.
- Require `RedeemTypeInvitation`.
- Require the code to be usable or unused according to the local redeem-code model.
- Return the redeem code so the caller can consume it after user creation.

Update all new-user creation paths to call this helper unconditionally, instead of guarding it with `IsInvitationCodeEnabled`.

When a user is created, consume the invitation code with the new user ID. Where a transaction is already available, keep user creation and invitation consumption in the same transaction. Where the existing path is not transactional, preserve the current behavior and return/log errors consistently with nearby code.

## Frontend Design

The registration page should always render the invitation-code input. It should be presented as required, not optional, and submission should fail client-side when it is blank.

The frontend can keep using `validateInvitationCode` for early feedback. If that endpoint currently returns `INVITATION_CODE_DISABLED` when the setting is off, update the endpoint so validation works regardless of the setting.

`RegisterRequest.invitation_code` should become a required field in TypeScript. Call sites that complete OAuth registration should continue sending the invitation code they already collect.

## Error Handling

Use existing error codes where possible:

- Empty email registration invitation code: `INVITATION_CODE_REQUIRED`.
- Invalid, non-invitation, used, or expired invitation code: `INVITATION_CODE_INVALID`.
- OAuth flows that intentionally need the pending-completion state can continue using `OAUTH_INVITATION_REQUIRED` when missing an invitation code.

Frontend messages should continue mapping these states to the existing invitation-code validation copy.

## Testing

Backend unit tests should cover:

- Email registration without invitation code fails.
- Email registration with a valid invitation code succeeds and consumes the code.
- Email registration with a used or non-invitation code fails.
- OAuth new-user registration without invitation code fails.
- OAuth existing-user login does not require an invitation code.
- Public invitation-code validation works even when `invitation_code_enabled` is false.

Frontend tests should cover:

- The invitation-code field is rendered regardless of public `invitation_code_enabled`.
- Register submission is blocked when the invitation code is blank.
- Register payload includes `invitation_code` when submitted.

## Rollout Notes

Before deploying, ensure production has enough unused invitation redeem codes. After deployment, public registration without a valid invitation code will fail even if the admin setting `invitation_code_enabled` is false.
