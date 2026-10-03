import axios, { type AxiosInstance } from 'axios'

/**
 * 管理端 API 客户端（M0 骨架）。
 *
 * 约定：
 *   - 后端统一信封 `{ code, data, msg }`，code !== 0 视为业务错误；
 *   - 管理端会话走 HttpOnly Cookie（后端签发），**不把凭据放进 localStorage**；
 *   - 后台所有写操作后端都会记 `audit_log`，前端不得自行组装审计字段。
 */
export const http: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 20000,
  withCredentials: true,
  headers: { Accept: 'application/json' },
})

http.interceptors.response.use(
  (res) => res,
  (err) => {
    // 401（未认证）与 403（已认证但无权限）语义区分，便于排查越权问题
    const status = err?.response?.status
    if (status === 401) {
      // M1 起跳转登录；当前骨架只记录
      console.warn('[cw-tc] 未认证，需登录')
    } else if (status === 403) {
      console.warn('[cw-tc] 无权限（已认证但角色/资源归属不匹配）')
    }
    return Promise.reject(err)
  },
)

/** 健康检查：与后端 `GET /api/v1/health` 对应。 */
export async function health() {
  const { data } = await http.get('/health')
  return data
}
