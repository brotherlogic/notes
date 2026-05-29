import React, { useState, useEffect } from 'react';
import NotebookDashboard from './components/NotebookDashboard';
import PageViewer from './components/PageViewer';

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [selectedNotebook, setSelectedNotebook] = useState(null);
  const [isLoading, setIsLoading] = useState(true);

  // Check login state on mount
  useEffect(() => {
    // If the browser has a 'notes_session' cookie, fetch user config to verify
    fetch('/api/user/config')
      .then(res => {
        if (res.ok) {
          setIsAuthenticated(true);
        } else {
          setIsAuthenticated(false);
        }
        setIsLoading(false);
      })
      .catch(() => {
        setIsAuthenticated(false);
        setIsLoading(false);
      });
  }, []);

  const handleLogin = () => {
    // Redirect to GitHub OAuth flow endpoint on backend
    window.location.href = '/login/github';
  };

  const handleLogout = () => {
    fetch('/api/logout', { method: 'POST' })
      .then(res => {
        if (res.ok) {
          setIsAuthenticated(false);
          setSelectedNotebook(null);
        } else {
          console.error("Failed to log out");
        }
      })
      .catch(err => console.error("Error during logout:", err));
  };

  if (isLoading) {
    return (
      <div style={{
        height: '100vh',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        color: 'var(--text-secondary)'
      }}>
        Loading notes space...
      </div>
    );
  }

  // Render Premium Landing Login Screen if unauthenticated
  if (!isAuthenticated) {
    return (
      <div style={{
        height: '100vh',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '20px'
      }}>
        <div
          className="glass-container"
          style={{
            maxWidth: '440px',
            width: '100%',
            padding: '40px',
            borderRadius: '24px',
            textAlign: 'center'
          }}
        >
          <div style={{
            fontSize: '3.5rem',
            marginBottom: '20px',
            backgroundImage: 'linear-gradient(135deg, var(--accent), var(--text-primary))',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            fontWeight: 800,
            letterSpacing: '-0.03em'
          }}>
            Notes
          </div>
          
          <h2 style={{ fontSize: '1.5rem', fontWeight: 600, marginBottom: '12px' }}>
             handwritten notes organized.
          </h2>
          
          <p style={{ color: 'var(--text-secondary)', fontSize: '0.95rem', marginBottom: '32px', lineHeight: 1.5 }}>
            Log in with GitHub, connect your personal Google Drive notebooks folder, and file crops directly as repository issues.
          </p>

          <button
            onClick={handleLogin}
            className="btn btn-primary"
            style={{ width: '100%', justifyContent: 'center', padding: '12px 24px', fontSize: '1rem', borderRadius: '12px' }}
          >
            🐙 Log In with GitHub
          </button>
        </div>
      </div>
    );
  }

  // Render dashboard or page viewer when logged in
  return (
    <div style={{ minHeight: '100vh' }}>
      {selectedNotebook ? (
        <PageViewer
          notebook={selectedNotebook}
          onClose={() => setSelectedNotebook(null)}
        />
      ) : (
        <NotebookDashboard
          onSelectNotebook={(nb) => setSelectedNotebook(nb)}
          activeNotebookId={selectedNotebook?.id}
          onLogout={handleLogout}
        />
      )}
    </div>
  );
}
