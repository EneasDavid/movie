import { useState } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { ApiError } from "../lib/api";
import PasswordField from "../components/auth/PasswordField";

export default function Login() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  const params = new URLSearchParams(location.search);
  const verifyStatus = params.get("verify");
  const resetStatus = params.get("reset");

  if (user) return <Navigate to="/" replace />;

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(email, password);
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.body?.pendingVerification) {
        navigate("/check-email", { state: { email: err.body.email || email } });
        return;
      }
      setError(err instanceof ApiError ? err.message : "Erro ao entrar");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <a className="brand auth-brand" href="/">df-orfeu</a>
        <h1 className="auth-title">Entrar</h1>

        {verifyStatus === "success" && (
          <p className="auth-notice">Email confirmado — já pode entrar.</p>
        )}
        {verifyStatus === "invalid" && (
          <p className="auth-notice auth-notice-error">Link de confirmação inválido ou expirado.</p>
        )}
        {verifyStatus === "error" && (
          <p className="auth-notice auth-notice-error">Não foi possível confirmar seu email. Tente novamente.</p>
        )}
        {resetStatus === "success" && (
          <p className="auth-notice">Senha redefinida — já pode entrar com a nova senha.</p>
        )}

        <form className="auth-form" onSubmit={handleSubmit}>
          <label className="auth-label">
            Email
            <input
              className="auth-input"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>
          <PasswordField
            label="Senha"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />

          {error && <p className="auth-notice auth-notice-error">{error}</p>}

          <button className="btn btn-primary auth-submit" type="submit" disabled={submitting}>
            {submitting ? "Entrando…" : "Entrar"}
          </button>
        </form>

        <p className="auth-switch">
          <Link to="/forgot-password">Esqueci minha senha</Link>
        </p>
        <p className="auth-switch">
          Não tem conta? <Link to="/signup">Criar conta</Link>
        </p>
      </div>
    </div>
  );
}
