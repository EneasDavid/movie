import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import Avatar from "../components/auth/Avatar";
import Row from "../components/catalog/Row";
import { ApiError, fetchProgress, removeProgress } from "../lib/api";
import { MIN_RESUMABLE_SECONDS, NEAR_END_RATIO } from "../lib/progress";
import { passwordChecklist, isPasswordValid } from "../lib/passwordRules";

const MAX_PHOTO_BYTES = 3 * 1024 * 1024;

export default function Profile() {
  const { user, uploadAvatar, updateProfile, changePassword } = useAuth();
  const fileInputRef = useRef(null);

  // --- Nome ---
  const [firstName, setFirstName] = useState(user?.firstName || "");
  const [lastName, setLastName] = useState(user?.lastName || "");
  const [profileError, setProfileError] = useState(null);
  const [profileNotice, setProfileNotice] = useState(null);
  const [savingProfile, setSavingProfile] = useState(false);

  // --- Foto ---
  const [avatarError, setAvatarError] = useState(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);

  // --- Senha ---
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmNewPassword, setConfirmNewPassword] = useState("");
  const [passwordError, setPasswordError] = useState(null);
  const [passwordNotice, setPasswordNotice] = useState(null);
  const [savingPassword, setSavingPassword] = useState(false);

  // --- Continuar assistindo ---
  const [progressEntries, setProgressEntries] = useState([]);
  const [progressLoading, setProgressLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    fetchProgress()
      .then((entries) => {
        if (!cancelled) setProgressEntries(entries);
      })
      .catch(() => {
        /* same as the catalog row — a failure here just means an empty section, not an error banner */
      })
      .finally(() => {
        if (!cancelled) setProgressLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const checklist = passwordChecklist(newPassword);

  const handleProfileSubmit = async (e) => {
    e.preventDefault();
    setProfileError(null);
    setProfileNotice(null);
    setSavingProfile(true);
    try {
      await updateProfile(firstName.trim(), lastName.trim());
      setProfileNotice("Nome atualizado.");
    } catch (err) {
      setProfileError(err instanceof ApiError ? err.message : "Erro ao atualizar o perfil");
    } finally {
      setSavingProfile(false);
    }
  };

  const handlePhotoChange = async (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > MAX_PHOTO_BYTES) {
      setAvatarError("A foto precisa ter no máximo 3MB.");
      return;
    }
    setAvatarError(null);
    setUploadingAvatar(true);
    try {
      await uploadAvatar(file);
    } catch (err) {
      setAvatarError(err instanceof ApiError ? err.message : "Erro ao enviar a foto");
    } finally {
      setUploadingAvatar(false);
      e.target.value = "";
    }
  };

  const handlePasswordSubmit = async (e) => {
    e.preventDefault();
    setPasswordError(null);
    setPasswordNotice(null);

    if (!isPasswordValid(newPassword)) {
      setPasswordError("A senha ainda não atende todos os requisitos abaixo.");
      return;
    }
    if (newPassword !== confirmNewPassword) {
      setPasswordError("As senhas não coincidem.");
      return;
    }

    setSavingPassword(true);
    try {
      await changePassword(currentPassword, newPassword);
      setPasswordNotice("Senha alterada.");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmNewPassword("");
    } catch (err) {
      setPasswordError(err instanceof ApiError ? err.message : "Erro ao trocar a senha");
    } finally {
      setSavingPassword(false);
    }
  };

  const handleRemoveFromContinueWatching = useCallback((fileId) => {
    setProgressEntries((prev) => prev.filter((e) => e.fileId !== fileId));
    removeProgress(fileId).catch(() => {
      /* best-effort, same as the catalog row */
    });
  }, []);

  const continueWatchingItems = progressEntries
    .filter((e) => e.time >= MIN_RESUMABLE_SECONDS && !(e.duration > 0 && e.time / e.duration >= NEAR_END_RATIO))
    .map((e) => ({ id: e.fileId, name: e.title || "" }));

  const progressByFileId = new Map(progressEntries.map((e) => [e.fileId, { time: e.time, duration: e.duration }]));

  return (
    <div className="profile-page">
      <header className="topbar">
        <Link className="brand" to="/">df-orfeu</Link>
        <Link className="btn btn-primary profile-back" to="/">Voltar ao catálogo</Link>
      </header>

      <main className="profile-main">
        <h1 className="profile-title">Sua conta</h1>

        <section className="profile-section">
          <h2 className="profile-section-title">Perfil</h2>
          <div className="profile-photo-row">
            <button
              type="button"
              className="auth-photo-picker profile-avatar-picker"
              onClick={() => fileInputRef.current?.click()}
              aria-label="Trocar foto de perfil"
              disabled={uploadingAvatar}
            >
              <Avatar hasAvatar={user?.hasAvatar} size={72} />
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              hidden
              onChange={handlePhotoChange}
            />
            <div>
              <button
                type="button"
                className="profile-photo-link"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploadingAvatar}
              >
                {uploadingAvatar ? "Enviando…" : "Trocar foto"}
              </button>
              <p className="auth-photo-hint">JPEG, PNG ou WebP, até 3MB.</p>
            </div>
          </div>
          {avatarError && <p className="auth-notice auth-notice-error">{avatarError}</p>}

          <form className="auth-form profile-form" onSubmit={handleProfileSubmit}>
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
              <input className="auth-input" type="email" value={user?.email || ""} disabled />
            </label>
            {profileError && <p className="auth-notice auth-notice-error">{profileError}</p>}
            {profileNotice && <p className="auth-notice">{profileNotice}</p>}
            <button className="btn btn-primary auth-submit" type="submit" disabled={savingProfile}>
              {savingProfile ? "Salvando…" : "Salvar nome"}
            </button>
          </form>
        </section>

        <section className="profile-section">
          <h2 className="profile-section-title">Trocar senha</h2>
          <form className="auth-form profile-form" onSubmit={handlePasswordSubmit}>
            <label className="auth-label">
              Senha atual
              <input
                className="auth-input"
                type="password"
                autoComplete="current-password"
                required
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
              />
            </label>
            <label className="auth-label">
              Nova senha
              <input
                className="auth-input"
                type="password"
                autoComplete="new-password"
                required
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
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
                value={confirmNewPassword}
                onChange={(e) => setConfirmNewPassword(e.target.value)}
              />
            </label>
            {passwordError && <p className="auth-notice auth-notice-error">{passwordError}</p>}
            {passwordNotice && <p className="auth-notice">{passwordNotice}</p>}
            <button className="btn btn-primary auth-submit" type="submit" disabled={savingPassword}>
              {savingPassword ? "Salvando…" : "Trocar senha"}
            </button>
          </form>
        </section>

        <section className="profile-section">
          <h2 className="profile-section-title">Continuar assistindo</h2>
          {!progressLoading && continueWatchingItems.length === 0 && (
            <p className="profile-empty">Nada em andamento no momento.</p>
          )}
          {continueWatchingItems.length > 0 && (
            <div className="profile-continue-row">
              <Row
                title=""
                items={continueWatchingItems}
                progressByFileId={progressByFileId}
                onRemoveItem={handleRemoveFromContinueWatching}
              />
            </div>
          )}
        </section>
      </main>
    </div>
  );
}
