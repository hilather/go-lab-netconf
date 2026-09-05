import { type ReactNode } from "react";
import { BrowserRouter, NavLink, Navigate, Outlet, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./auth/AuthProvider";
import { SCOPE_ADMIN, SCOPE_AUDIT } from "./auth/scopes";
import { ApplyPage } from "./pages/ApplyPage";
import { AuditPage } from "./pages/AuditPage";
import { DatastoresPage } from "./pages/DatastoresPage";
import { LoginPage } from "./pages/LoginPage";
import { NotificationsPage } from "./pages/NotificationsPage";
import { OverviewPage } from "./pages/OverviewPage";
import { ProfilesPage } from "./pages/ProfilesPage";
import { SessionsPage } from "./pages/SessionsPage";
import { StatePage } from "./pages/StatePage";
import { UsersPage } from "./pages/UsersPage";
import { navItems } from "./ui/nav";

function SkipLink() {
  return (
    <a className="skip-link" href="#app-main">
      Skip to main content
    </a>
  );
}

function NavItem({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink to={to} className={({ isActive }) => (isActive ? "nav-active" : undefined)} end={to === "/"}>
      {children}
    </NavLink>
  );
}

function Shell() {
  const { state, hasScope, logout } = useAuth();
  const signedIn = state.status === "signed_in";
  const items = signedIn ? navItems(hasScope(SCOPE_ADMIN), hasScope(SCOPE_AUDIT)) : [];
  return (
    <div className="app">
      <SkipLink />
      <header className="topbar">
        <NavLink className="brand" to="/">
          LabNETCONF
        </NavLink>
        <nav aria-label="Primary">
          {items.map((item) => (
            <NavItem key={item.to} to={item.to}>
              {item.label}
            </NavItem>
          ))}
          {signedIn ? (
            <button type="button" className="linkish" onClick={() => void logout()}>
              Sign out
            </button>
          ) : null}
        </nav>
      </header>
      <div id="app-main">
        <Outlet />
      </div>
    </div>
  );
}

function RequireSession() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status !== "signed_in") {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}

function RedirectIfSignedIn() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status === "signed_in") {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route element={<Shell />}>
            <Route element={<RedirectIfSignedIn />}>
              <Route path="/login" element={<LoginPage />} />
            </Route>
            <Route element={<RequireSession />}>
              <Route path="/" element={<OverviewPage />} />
              <Route path="/state" element={<StatePage />} />
              <Route path="/profiles" element={<ProfilesPage />} />
              <Route path="/users" element={<UsersPage />} />
              <Route path="/datastores" element={<DatastoresPage />} />
              <Route path="/notifications" element={<NotificationsPage />} />
              <Route path="/sessions" element={<SessionsPage />} />
              <Route path="/apply" element={<ApplyPage />} />
              <Route path="/audit" element={<AuditPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
