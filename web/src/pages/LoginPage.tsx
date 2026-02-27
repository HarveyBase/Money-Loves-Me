import { useState, FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { authAPI } from '../services/api'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const navigate = useNavigate()

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      const res = await authAPI.login(username, password)
      localStorage.setItem('token', res.data.token)
      navigate('/')
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
      setError(msg || '登录失败')
    }
  }

  return (
    <div style={{ maxWidth: 400, margin: '100px auto', padding: 20 }}>
      <h1>币安交易系统</h1>
      <form onSubmit={handleSubmit}>
        <div style={{ marginBottom: 12 }}>
          <label htmlFor="username">用户名</label>
          <input id="username" type="text" value={username}
            onChange={(e) => setUsername(e.target.value)} required
            style={{ display: 'block', width: '100%', padding: 8 }} />
        </div>
        <div style={{ marginBottom: 12 }}>
          <label htmlFor="password">密码</label>
          <input id="password" type="password" value={password}
            onChange={(e) => setPassword(e.target.value)} required
            style={{ display: 'block', width: '100%', padding: 8 }} />
        </div>
        {error && <p style={{ color: 'red' }} role="alert">{error}</p>}
        <button type="submit" style={{ padding: '8px 24px' }}>登录</button>
      </form>
    </div>
  )
}
