import { useState } from "react";
import { Link } from "react-router-dom";
import { forgotPassword } from "../lib/api";

export default function ForgotPassword() {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      await forgotPassword(email);
    } catch {
      // Deliberately ignored: the endpoint always responds 204 regardless
      // of whether the email exists, so a network-level failure is the
      // only realistic error here, and retrying/showing the same
      // generic message either way is correct — never reveal whether an
      // account exists.
    } finally {
      setSubmitting(false);
      setSent(true);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <a className="brand auth-brand" href="/">DF Orfeu</a>
        <h1 className="auth-title">Esqueci minha senha</h1>

        {sent ? (
          <p className="auth-notice">
            Se existe uma conta com esse email, enviamos um link para redefinir a senha. O link expira em
            30 minutos.
          </p>
        ) : (
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
            <button className="btn btn-primary auth-submit" type="submit" disabled={submitting}>
              {submitting ? "Enviando…" : "Enviar link"}
            </button>
          </form>
        )}

        <p className="auth-switch">
          <Link to="/login">Voltar ao login</Link>
        </p>
      </div>
    </div>
  );
}
