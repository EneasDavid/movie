import { useState } from "react";
import { Link, Navigate, useLocation } from "react-router-dom";
import { resendVerification } from "../lib/api";

// Landing spot after signup, or after a login attempt on an unverified
// account — same screen either way, since both mean the same thing:
// "there's a confirmation link in your inbox, and you can't get in
// without clicking it."
export default function CheckEmail() {
  const location = useLocation();
  const email = location.state?.email;
  const [resendState, setResendState] = useState("idle"); // idle | sending | sent | error

  // Reached directly (refresh, bookmark) with no email in state — there's
  // nothing useful to show, send them to log in instead of a dead end.
  if (!email) return <Navigate to="/login" replace />;

  const handleResend = async () => {
    setResendState("sending");
    try {
      await resendVerification(email);
      setResendState("sent");
    } catch {
      setResendState("error");
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <a className="brand auth-brand" href="/">df-orfeu</a>
        <h1 className="auth-title">Confirme seu email</h1>

        <p className="auth-notice">
          Enviamos um link de confirmação para <strong>{email}</strong>. Clique nele para poder entrar —
          o link expira em 30 minutos.
        </p>

        {resendState === "sent" ? (
          <p className="auth-notice">Reenviado — confira sua caixa de entrada (e o spam).</p>
        ) : (
          <button
            className="btn btn-primary auth-submit"
            type="button"
            onClick={handleResend}
            disabled={resendState === "sending"}
          >
            {resendState === "sending" ? "Reenviando…" : "Reenviar link"}
          </button>
        )}
        {resendState === "error" && (
          <p className="auth-notice auth-notice-error">Não foi possível reenviar agora. Tente novamente em instantes.</p>
        )}

        <p className="auth-switch">
          <Link to="/login">Voltar ao login</Link>
        </p>
      </div>
    </div>
  );
}
