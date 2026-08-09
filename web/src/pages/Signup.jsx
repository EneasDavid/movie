import { useRef, useState } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { ApiError } from "../lib/api";
import { passwordChecklist, isPasswordValid } from "../lib/passwordRules";
import PasswordField from "../components/auth/PasswordField";

const MAX_PHOTO_BYTES = 3 * 1024 * 1024;

export default function Signup() {
  const { user, signup } = useAuth();
  const navigate = useNavigate();
  const fileInputRef = useRef(null);

  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [photoPreview, setPhotoPreview] = useState(null);
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  if (user) return <Navigate to="/" replace />;

  const checklist = passwordChecklist(password);

  const handlePhotoChange = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > MAX_PHOTO_BYTES) {
      setError("A foto precisa ter no máximo 3MB.");
      return;
    }
    setError(null);
    // Preview-only for now: uploading requires a session, which doesn't
    // exist until after the account is verified and logged in.
    setPhotoPreview(URL.createObjectURL(file));
  };

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
      const result = await signup(email, password, firstName.trim(), lastName.trim());
      if (result?.emailSent === false) {
        setError("A conta foi criada, mas não foi possível enviar o email de confirmação. Tente entrar novamente depois que o domínio de email for ativado.");
        return;
      }
      // Signup never starts a session (login requires a verified email,
      // and a fresh account never is yet) — send them to check their
      // inbox instead of the catalog. The chosen photo isn't uploaded
      // yet either, for the same reason (upload requires a session);
      // it can be added from the profile once logged in.
      navigate("/check-email", { state: { email: result?.email || email } });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Erro ao criar conta");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <a className="brand auth-brand" href="/">DF Orfeu</a>
        <h1 className="auth-title">Criar conta</h1>

        <form className="auth-form" onSubmit={handleSubmit}>
          <div className="auth-photo-row">
            <button
              type="button"
              className="auth-photo-picker"
              onClick={() => fileInputRef.current?.click()}
              aria-label="Escolher foto de perfil"
            >
              {photoPreview ? (
                <img className="auth-photo-preview" src={photoPreview} alt="" />
              ) : (
                <span className="auth-photo-placeholder">+ Foto</span>
              )}
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              hidden
              onChange={handlePhotoChange}
            />
            <p className="auth-photo-hint">
              Opcional — sem foto, usamos um ícone padrão.
            </p>
          </div>

          <div className="auth-name-row">
            <label className="auth-label">
              Nome
              <input
                className="auth-input"
                type="text"
                autoComplete="given-name"
                required
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
              />
            </label>
            <label className="auth-label">
              Sobrenome
              <input
                className="auth-input"
                type="text"
                autoComplete="family-name"
                required
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
              />
            </label>
          </div>

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
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />

          <ul className="password-checklist">
            {checklist.map((rule) => (
              <li key={rule.key} className={rule.ok ? "ok" : ""}>
                <span className="password-checklist-icon" aria-hidden="true">{rule.ok ? "✓" : "○"}</span>
                {rule.label}
              </li>
            ))}
          </ul>

          <PasswordField
            label="Confirmar senha"
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
          />

          {error && <p className="auth-notice auth-notice-error">{error}</p>}

          <button className="btn btn-primary auth-submit" type="submit" disabled={submitting}>
            {submitting ? "Criando conta…" : "Criar conta"}
          </button>
        </form>

        <p className="auth-switch">
          Já tem conta? <Link to="/login">Entrar</Link>
        </p>
      </div>
    </div>
  );
}
