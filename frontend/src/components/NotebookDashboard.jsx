import React, { useState, useEffect } from 'react';

export default function NotebookDashboard({ onSelectNotebook, activeNotebookId, onLogout }) {
  const [userConfig, setUserConfig] = useState(null);
  const [notebooks, setNotebooks] = useState([]);
  const [folderIdInput, setFolderIdInput] = useState('');
  const [isLoading, setIsLoading] = useState(true);

  // Fetch initial configs and notebooks
  useEffect(() => {
    fetch('/api/user/config')
      .then(res => res.json())
      .then(data => {
        setUserConfig(data);
        setFolderIdInput(data.gdriveNotesFolderId || '');
      })
      .catch(err => console.error("Error fetching user config:", err));

    fetch('/api/notebooks')
      .then(res => res.json())
      .then(data => {
        setNotebooks(data || []);
        setIsLoading(false);
      })
      .catch(err => {
        console.error("Error fetching notebooks:", err);
        setIsLoading(false);
      });
  }, []);

  const handleLinkGDrive = () => {
    // Redirect to backend Google OAuth flow
    window.location.href = '/link/gdrive';
  };

  const handleSaveFolder = (e) => {
    e.preventDefault();
    fetch('/api/config/folder', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ folder_id: folderIdInput })
    })
      .then(res => {
        if (res.ok) {
          alert('Google Drive notes folder configured successfully!');
        } else {
          alert('Failed to configure folder.');
        }
      })
      .catch(err => console.error(err));
  };

  return (
    <div style={{ padding: '24px', maxWidth: '1200px', margin: '0 auto' }}>
      {/* Top Banner */}
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '32px' }}>
        <div>
          <h1 style={{ fontSize: '2.5rem', fontWeight: 700, letterSpacing: '-0.025em', color: 'var(--text-primary)' }}>
            Supernotes Visualizer
          </h1>
          <p style={{ color: 'var(--text-secondary)', marginTop: '4px' }}>
            Sync, crop, and file GitHub issues from your handwritten notes.
          </p>
        </div>

        {/* Integration Badges */}
        <div style={{ display: 'flex', gap: '12px' }}>
          <div className="glass-container" style={{ padding: '8px 16px', display: 'flex', alignItems: 'center', gap: '8px', borderRadius: '12px' }}>
            <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: 'var(--success)' }}></span>
            <span style={{ fontSize: '0.875rem', fontWeight: 500 }}>
              {userConfig?.githubUsername ? `@${userConfig.githubUsername}` : 'GitHub Connected'}
            </span>
          </div>

          <button
            onClick={handleLinkGDrive}
            className="btn"
            style={{
              backgroundColor: userConfig?.gdriveRefreshToken ? 'var(--success-glow)' : 'var(--bg-surface)',
              color: userConfig?.gdriveRefreshToken ? 'var(--success)' : 'var(--text-primary)',
              border: '1px solid var(--border-frosted)',
              borderRadius: '12px',
              cursor: 'pointer'
            }}
          >
            {userConfig?.gdriveRefreshToken ? '✓ GDrive Connected' : '🔌 Link Google Drive'}
          </button>

          <button
            onClick={onLogout}
            className="btn"
            style={{
              backgroundColor: 'rgba(239, 68, 68, 0.1)',
              color: '#ef4444',
              border: '1px solid rgba(239, 68, 68, 0.2)',
              borderRadius: '12px',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              transition: 'var(--transition-smooth)'
            }}
            onMouseOver={(e) => {
              e.currentTarget.style.backgroundColor = 'rgba(239, 68, 68, 0.2)';
              e.currentTarget.style.boxShadow = '0 0 15px rgba(239, 68, 68, 0.2)';
            }}
            onMouseOut={(e) => {
              e.currentTarget.style.backgroundColor = 'rgba(239, 68, 68, 0.1)';
              e.currentTarget.style.boxShadow = 'none';
            }}
          >
            🚪 Log Out
          </button>
        </div>
      </header>

      {/* Directory configuration panel */}
      {userConfig && (
        <section className="glass-container" style={{ padding: '20px', marginBottom: '40px', borderRadius: '16px' }}>
          <h3 style={{ marginBottom: '12px', fontWeight: 600 }}>Configure Notebooks Location</h3>
          <form onSubmit={handleSaveFolder} style={{ display: 'flex', gap: '12px', maxWidth: '600px' }}>
            <input
              type="text"
              placeholder="Enter Google Drive Folder ID containing your notebook PDFs"
              value={folderIdInput}
              onChange={(e) => setFolderIdInput(e.target.value)}
              style={{
                flex: 1,
                padding: '10px 16px',
                borderRadius: '8px',
                border: '1px solid var(--border-frosted)',
                backgroundColor: 'rgba(255, 255, 255, 0.05)',
                color: '#fff',
                outline: 'none',
                fontFamily: 'var(--font-body)'
              }}
            />
            <button type="submit" className="btn btn-primary">Save Config</button>
          </form>
        </section>
      )}

      {/* Notebook Grid */}
      <section>
        <h2 style={{ fontSize: '1.75rem', fontWeight: 600, marginBottom: '20px' }}>Your Notebooks</h2>
        {isLoading ? (
          <div style={{ color: 'var(--text-secondary)' }}>Scanning notebooks...</div>
        ) : notebooks.length === 0 ? (
          <div className="glass-container" style={{ padding: '40px', textAlign: 'center', color: 'var(--text-secondary)' }}>
            No synced notebooks found. Ensure your GDrive folder is linked and has synced pages.
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '24px' }}>
            {notebooks.map(nb => {
              const isActive = activeNotebookId === nb.id;
              return (
                <div
                  key={nb.id}
                  onClick={() => onSelectNotebook(nb)}
                  className="glass-container"
                  style={{
                    padding: '24px',
                    borderRadius: '16px',
                    cursor: 'pointer',
                    borderColor: isActive ? 'var(--accent)' : 'var(--border-frosted)',
                    boxShadow: isActive ? '0 12px 40px 0 var(--accent-glow)' : 'var(--shadow-frosted)',
                    transition: 'var(--transition-smooth)'
                  }}
                >
                  <h3 style={{ fontSize: '1.25rem', marginBottom: '8px', fontWeight: 600 }}>{nb.title || 'Untitled Notebook'}</h3>
                  <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '16px' }}>
                    📦 {nb.pages?.length || 0} Synced Pages
                  </p>
                  <span style={{ fontSize: '0.875rem', color: 'var(--accent)', fontWeight: 500 }}>
                    Open Notebook →
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
