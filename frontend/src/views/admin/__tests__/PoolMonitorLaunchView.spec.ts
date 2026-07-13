import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PoolMonitorLaunchView from '../PoolMonitorLaunchView.vue'
import { createPoolMonitorTicket } from '@/api/admin/poolMonitor'

vi.mock('@/api/admin/poolMonitor', () => ({
  createPoolMonitorTicket: vi.fn()
}))

describe('PoolMonitorLaunchView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('通过 POST 表单提交票据且不把票据放进 URL', async () => {
    vi.mocked(createPoolMonitorTicket).mockResolvedValue({
      target_url: 'https://pool.example.invalid/api/admin/v1/auth/exchange',
      ticket: 'SIGNED_TICKET_FIXTURE'
    })
    let submittedForm: HTMLFormElement | undefined
    vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(function () {
      submittedForm = this
    })
    const wrapper = mount(PoolMonitorLaunchView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } }
    })

    await wrapper.get('[data-test="open-standalone"]').trigger('click')
    await flushPromises()

    expect(createPoolMonitorTicket).toHaveBeenCalledWith('standalone')
    expect(submittedForm).toBeDefined()
    expect(submittedForm?.method).toBe('post')
    expect(submittedForm?.action).toBe('https://pool.example.invalid/api/admin/v1/auth/exchange')
    expect(new FormData(submittedForm).get('ticket')).toBe('SIGNED_TICKET_FIXTURE')
    expect(submittedForm?.action).not.toContain('SIGNED_TICKET_FIXTURE')
  })

  it('请求失败时停留在当前页并显示错误', async () => {
    vi.mocked(createPoolMonitorTicket).mockRejectedValue(new Error('fixture failure'))
    const wrapper = mount(PoolMonitorLaunchView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } }
    })

    await wrapper.get('[data-test="open-integrated"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="launch-error"]').text()).toContain('暂时无法进入')
  })
})
