import React, { useState, useEffect } from 'react';

export default function GitHubRepoModal({ notebook, isOpen, onClose, onSaved }) {
  const [repoInput, setRepoInput] = useState('');
  const [repos, setRepos] = useState([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [isLoadingRepos, setIsLoadingRepos] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isOpen || !notebook) return;
    const currentRepo = notebook.githubProject || notebook.github_project || '';
    setRepoInput(currentRepo);
    setSearchQuery('');
    setError('');
    setIsLoadingRepos(true);

    fetch('/api/github/repos')
      .then(res => {
        if (!res.ok) throw new Error('Could not load repositories from GitHub.');
        return res.json();
      })
      .then(data => {
        setRepos(Array.isArray(data) ? data : []);
        setIsLoadingRepos(false);
      })
      .catch(err => {
        console.error('Error loading GitHub repos:', err);
        setRepos([]);
        setIsLoadingRepos(false);
      });
  }, [isOpen, notebook]);

  if (!isOpen || !notebook) return null;

  const saveRepo = (targetRepo) => {
    const trimmed = targetRepo.trim();
    if (trimmed !== '' && !/^[a-zA-Z0-9_.-]+\/[a-zA-Z0-9_.-]+$/.test(trimmed)) {
      setError('Repository must be in "owner/repo" format.');
      return;
    }

    setIsSaving(true);
    setError('');

    fetch('/api/notebooks/github-project', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        notebook_id: notebook.id,
        github_project: trimmed
      })
    })
      .then(async res => {
        if (!res.ok) {
          const text = await res.text();
          throw new Error(text || 'Failed to update GitHub repository link');
        }
        return res.json();
      })
      .then(updatedNotebook => {
        setIsSaving(false);
        if (onSaved) {
          onSaved(updatedNotebook);
        }
        onClose();
      })
      .catch(err => {
        console.error('Error saving GitHub project:', err);
        setError(err.message || 'Failed to save GitHub repository link.');
        setIsSaving(false);
      });
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    saveRepo(repoInput);
  };

  const filteredRepos = repos.filter(r =>
    (r.full_name || r.name || '').toLowerCase().includes(searchQuery.toLowerCase())
  );

  const currentLinked = notebook.githubProject || notebook.github_project || '';

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        backdropFilter: 'blur(12px)',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        zIndex: 1100
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="glass-container"
        style={{
          maxWidth: '520px',
          width: '90%',
          maxHeight: '82vh',
          padding: '28px',
          borderRadius: '24px',
          display: 'flex',
          flexDirection: 'column',
          gap: '18px',
          boxShadow: '0 20px 50px rgba(0, 0, 0, 0.45)',
          border: '1px solid var(--border-frosted)'
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h3 style={{ fontSize: '1.25rem', fontWeight: 600, margin: 0 }}>
              🐙 Link GitHub Repository
            </h3>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', margin: '4px 0 0 0' }}>
              Target repository for issues filed from <strong>{notebook.title || 'Untitled Notebook'}</strong>
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--text-secondary)',
              fontSize: '1.5rem',
              cursor: 'pointer',
              padding: '4px'
            }}
          >
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <label style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', fontWeight: 500 }}>
            Repository (<code>owner/repo</code>)
          </label>
          <input
            type="text"
            placeholder="e.g. brotherlogic/notes"
            value={repoInput}
            onChange={(e) => {
              setRepoInput(e.target.value);
              if (error) setError('');
            }}
            style={{
              width: '100%',
              boxSizing: 'border-box',
              padding: '10px 14px',
              borderRadius: '10px',
              border: '1px solid var(--border-frosted)',
              backgroundColor: 'rgba(255, 255, 255, 0.05)',
              color: '#fff',
              outline: 'none',
              fontFamily: 'var(--font-body)'
            }}
          />

          {error && (
            <div style={{
              fontSize: '0.8rem',
              color: '#fca5a5',
              padding: '8px 12px',
              borderRadius: '8px',
              backgroundColor: 'rgba(239, 68, 68, 0.1)',
              border: '1px solid rgba(239, 68, 68, 0.25)'
            }}>
              ⚠️ {error}
            </div>
          )}

          {/* Searchable GitHub Repository Picker */}
          <div style={{
            display: 'flex',
            flexDirection: 'column',
            gap: '8px',
            marginTop: '4px'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', fontWeight: 600 }}>
                Or Pick From Your GitHub Repositories
              </span>
            </div>

            <input
              type="text"
              placeholder="🔍 Filter your repositories..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{
                width: '100%',
                boxSizing: 'border-box',
                padding: '8px 12px',
                borderRadius: '8px',
                border: '1px solid var(--border-frosted)',
                backgroundColor: 'rgba(255, 255, 255, 0.03)',
                color: '#fff',
                outline: 'none',
                fontSize: '0.875rem',
                fontFamily: 'var(--font-body)'
              }}
            />

            <div style={{
              maxHeight: '200px',
              minHeight: '120px',
              overflowY: 'auto',
              display: 'flex',
              flexDirection: 'column',
              gap: '6px',
              paddingRight: '4px'
            }}>
              {isLoadingRepos ? (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1, color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                  Loading GitHub repositories...
                </div>
              ) : filteredRepos.length === 0 ? (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1, color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                  {repos.length === 0
                    ? 'No repositories loaded. Enter "owner/repo" manually above.'
                    : 'No matching repositories found.'}
                </div>
              ) : (
                filteredRepos.map(repo => {
                  const fullName = repo.full_name || repo.name;
                  const isSelected = repoInput.trim() === fullName;
                  return (
                    <div
                      key={repo.id || fullName}
                      onClick={() => {
                        setRepoInput(fullName);
                        if (error) setError('');
                      }}
                      className="glass-container"
                      style={{
                        padding: '10px 14px',
                        borderRadius: '10px',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: '10px',
                        backgroundColor: isSelected ? 'rgba(56, 189, 248, 0.12)' : 'transparent',
                        border: isSelected ? '1px solid var(--accent)' : '1px solid transparent',
                        transition: 'var(--transition-smooth)'
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                        <span>🐙</span>
                        <span style={{ fontWeight: 500, fontSize: '0.9rem', color: isSelected ? 'var(--accent)' : 'var(--text-primary)' }}>
                          {fullName}
                        </span>
                      </div>
                      {isSelected && (
                        <span style={{ fontSize: '0.75rem', color: 'var(--accent)', fontWeight: 600 }}>
                          Selected
                        </span>
                      )}
                    </div>
                  );
                })
              )}
            </div>
          </div>

          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '8px', gap: '10px' }}>
            <div>
              {currentLinked && (
                <button
                  type="button"
                  onClick={() => saveRepo('')}
                  disabled={isSaving}
                  className="btn btn-secondary"
                  style={{
                    color: '#fca5a5',
                    borderColor: 'rgba(239, 68, 68, 0.3)',
                    padding: '8px 14px',
                    fontSize: '0.85rem'
                  }}
                >
                  Unlink
                </button>
              )}
            </div>
            <div style={{ display: 'flex', gap: '10px' }}>
              <button
                type="button"
                onClick={onClose}
                disabled={isSaving}
                className="btn btn-secondary"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isSaving}
                className="btn btn-primary"
              >
                {isSaving ? 'Saving...' : '💾 Save Repository'}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
