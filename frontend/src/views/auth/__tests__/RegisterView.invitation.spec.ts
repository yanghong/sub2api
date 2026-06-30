import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import RegisterView from '../RegisterView.vue'

const getPublicSettings = vi.fn()
const validateInvitationCode = vi.fn()
const register = vi.fn()
const push = vi.fn()
const showError = vi.fn()
const showSuccess = vi.fn()
const showWarning = vi.fn()

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'en' }
    })
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push })
}))

vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: (...args: any[]) => getPublicSettings(...args),
    validateInvitationCode: (...args: any[]) => validateInvitationCode(...args)
  }
})

vi.mock('@/stores', () => ({
  useAuthStore: () => ({ register }),
  useAppStore: () => ({ showError, showSuccess, showWarning })
}))

function publicSettings(overrides: Record<string, unknown> = {}) {
  return {
    registration_enabled: true,
    email_verify_enabled: false,
    promo_code_enabled: false,
    invitation_code_enabled: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: 'Sub2API',
    linuxdo_oauth_enabled: false,
    wechat_oauth_enabled: false,
    oidc_oauth_enabled: false,
    github_oauth_enabled: false,
    google_oauth_enabled: false,
    registration_email_suffix_whitelist: [],
    ...overrides
  }
}

function mountRegisterView() {
  return mount(RegisterView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /></div>' },
        Icon: true,
        TurnstileWidget: true,
        LinuxDoOAuthSection: true,
        OidcOAuthSection: true,
        WechatOAuthSection: true,
        EmailOAuthButtons: true,
        LoginAgreementPrompt: true,
        RouterLink: true
      }
    }
  })
}

describe('RegisterView invitation code requirement', () => {
  beforeEach(() => {
    getPublicSettings.mockReset()
    validateInvitationCode.mockReset()
    register.mockReset()
    push.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    showWarning.mockReset()
    getPublicSettings.mockResolvedValue(publicSettings())
    validateInvitationCode.mockResolvedValue({ valid: true })
  })

  it('renders invitation code input even when public setting is disabled', async () => {
    const wrapper = mountRegisterView()
    await flushPromises()

    expect(wrapper.find('#invitation_code').exists()).toBe(true)
  })

  it('blocks submit when invitation code is blank', async () => {
    const wrapper = mountRegisterView()
    await flushPromises()

    await wrapper.find('#email').setValue('user@example.com')
    await wrapper.find('#password').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(register).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('auth.invitationCodeRequired')
  })

  it('submits invitation code in register payload', async () => {
    const wrapper = mountRegisterView()
    await flushPromises()

    await wrapper.find('#email').setValue('user@example.com')
    await wrapper.find('#password').setValue('secret-123')
    await wrapper.find('#invitation_code').setValue(' invite-ok ')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(register).toHaveBeenCalledWith(
      expect.objectContaining({
        email: 'user@example.com',
        password: 'secret-123',
        invitation_code: 'invite-ok'
      })
    )
  })
})
