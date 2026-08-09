import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { resetPassword, ApiError } from "../lib/api";
import { passwordChecklist, isPasswordValid } from "../lib/passwordRules";

export default function ResetPassword() {
  const [params] = useSearchParams();
  const token = params.get("token") || "";
  const navigate = useNavigate();

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  const checklist = passwordChecklist(password);

  if (!token) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <a className="brand auth-brand" href="/">df-orfeu</a>
          <p className="auth-notice auth-notice-error">Link inválido — faltando token de redefinição.</p>
          <p className="auth-switch">
            <Link to="/forgot-password">Solicitar novo link</Link>
          </p>
        </div>
      </div>
    );
  }

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);

    if (!isPasswordValid(password)) {
      setError("A senha ainda não atende todos os requisitos abaixo.");
      return;
    }
    if (password !== confirmPassword) {
      setError("As senhas não coincidem.");
      return;
    }

    setSubmitting(true);
    try {
      await resetPassword(token, password);
      navigate("/login?reset=success", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Erro ao redefinir a senha");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <a className="brand auth-brand" href="/">df-orfeu</a>
        <h1 className="auth-title">Redefinir senha</h1>

        <form className="auth-form" onSubmit={handleSubmit}>
          <label className="auth-label">
            Nova senha
            <input
              className="auth-input"
              type="password"
              autoComplete="new-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>

          <ul className="password-checklist">
            {checklist.map((rule) => (
              <li key={rule.key} className={rule.ok ? "ok" : ""}>
                <span className="password-checklist-icon" aria-hidden="true">{rule.ok ? "✓" : "○"}</span>
                {rule.label}
              </li>
            ))}
          </ul>

          <label className="auth-label">
            Confirmar nova senha
            <input
              className="auth-input"
              type="password"
              autoComplete="new-password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
            />
          </label>

          {error && <p className="auth-notice auth-notice-error">{error}</p>}

          <button className="btn btn-primary auth-submit" type="submit" disabled={submitting}>
            {submitting ? "Salvando…" : "Redefinir senha"}
          </button>
        </form>
      </div>
    </div>
  );
}
