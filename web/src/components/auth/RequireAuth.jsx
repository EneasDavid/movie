import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";

// Gate for "/" and "/watch": the login step is mandatory whenever there's
// no valid session — nothing else in the app renders until /api/auth/me
// has resolved one way or the other.
export default function RequireAuth({ children }) {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return <div className="auth-loading" aria-hidden="true" />;
  }
  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }
  return children;
}
