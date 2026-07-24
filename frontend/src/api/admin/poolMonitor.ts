import { apiClient } from '../client'

export type PoolMonitorTarget = 'integrated' | 'standalone'

export interface PoolMonitorTicketResponse {
  target_url: string
  ticket: string
}

export async function createPoolMonitorTicket(target: PoolMonitorTarget): Promise<PoolMonitorTicketResponse> {
  const { data } = await apiClient.post<PoolMonitorTicketResponse>('/admin/pool-monitor/sso-ticket', { target })
  return data
}
