import { useState, FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { authAPI } from '../services/api'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await authAPI.login(username, password)
      localStorage.setItem('token', res.data.token)
      navigate('/')
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
      setError(msg || '登录失败，请检查网络连接')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-wrapper">
      <div className="login-card">
        <div className="logo">
          <h1>◈ Money Loves Me</h1>
          <p>币安量化交易系统</p>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="username">用户名</label>
            <input id="username" type="text" className="form-input"
              placeholder="请输入用户名" value={username}
              onChange={(e) => setUsername(e.target.value)} required autoFocus />
          </div>
          <div className="form-group">
            <label htmlFor="password">密码</label>
            <input id="password" type="password" className="form-input"
              placeholder="请输入密码" value={password}
              onChange={(e) => setPassword(e.target.value)} required />
          </div>
          {error && <div className="error-msg" role="alert">{error}</div>}
          <button type="submit" className="btn btn-primary" disabled={loading}>
            {loading ? '登录中...' : '登 录'}
          </button>
        </form>
      </div>
    </div>
  )
}
