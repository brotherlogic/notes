import React, { useState, useEffect } from 'react';

export default function NotebookDashboard({ onSelectNotebook, activeNotebookId, onLogout }) {
  const [userConfig, setUserConfig] = useState(null);
  const [notebooks, setNotebooks] = useState([]);
  const [folderIdInput, setFolderIdInput] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [isFolderModalOpen, setIsFolderModalOpen] = useState(false);
  const [gdriveFolders, setGdriveFolders] = useState([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [isModalLoading, setIsModalLoading] = useState(false);
  const [syncStatus, setSyncStatus] = useState({ active: false, current: 0, total: 0, error: '' });

  const [folderDetails, setFolderDetails] = useState(null);
  const [selectedFolderPreview, setSelectedFolderPreview] = useState(null);
  const [isEditingFolder, setIsEditingFolder] = useState(false);
  const [isDetailsLoading, setIsDetailsLoading] = useState(false);

  // Fetch initial configs, notebooks, and active sync status
  useEffect(() => {
    fetch('/api/user/config')
      .then(res => res.json())
      .then(data => {
        setUserConfig(data);
        setFolderIdInput(data.gdriveNotesFolderId || '');
        if (data.gdriveNotesFolderId) {
          setIsDetailsLoading(true);
          fetch(`/api/gdrive/folder-details?folder_id=${data.gdriveNotesFolderId}`)
            .then(res => {
              if (res.ok) return res.json();
              throw new Error("Failed to fetch folder details");
            })
            .then(details => {
              setFolderDetails({
                id: details.folder_id,
                name: details.folder_name,
                file_count: details.file_count
              });
              setIsDetailsLoading(false);
            })
            .catch(err => {
              console.error(err);
              // Fallback to basic details
              setFolderDetails({
                id: data.gdriveNotesFolderId,
                name: "Configured Folder",
                file_count: 0
              });
              setIsDetailsLoading(false);
            });
        } else {
          setIsEditingFolder(true);
        }
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

    fetch('/api/sync/status')
      .then(res => res.json())
      .then(data => {
        if (data && data.active) {
          setSyncStatus(data);
        }
      })
      .catch(err => console.error("Error checking initial sync status:", err));
  }, []);

  // Poll sync status when active
  useEffect(() => {
    if (!syncStatus.active) return;

    const interval = setInterval(() => {
      fetch('/api/sync/status')
        .then(res => res.json())
        .then(data => {
          setSyncStatus(data || { active: false, current: 0, total: 0, error: '' });
          // If sync finished, refresh notebooks!
          if (data && !data.active) {
            fetch('/api/notebooks')
              .then(res => res.json())
              .then(nbs => {
                setNotebooks(nbs || []);
                // Refresh active folder details after sync finishes to update the file count
                if (userConfig?.gdriveNotesFolderId) {
                  fetch(`/api/gdrive/folder-details?folder_id=${userConfig.gdriveNotesFolderId}`)
                    .then(res => {
                      if (res.ok) return res.json();
                      throw new Error("Failed to fetch folder details after sync");
                    })
                    .then(details => {
                      setFolderDetails({
                        id: details.folder_id,
                        name: details.folder_name,
                        file_count: details.file_count
                      });
                    })
                    .catch(err => console.error(err));
                }
              })
              .catch(err => console.error("Error refreshing notebooks:", err));
          }
        })
        .catch(err => {
          console.error("Error polling sync status:", err);
        });
    }, 1000);

    return () => clearInterval(interval);
  }, [syncStatus.active, userConfig?.gdriveNotesFolderId]);

  const handleLinkGDrive = () => {
    // Redirect to backend Google OAuth flow
    window.location.href = '/link/gdrive';
  };

  const handleSaveFolder = (e) => {
    e.preventDefault();
    if (!folderIdInput) return;

    setIsLoading(true);
    fetch('/api/config/folder', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ folder_id: folderIdInput })
    })
      .then(res => {
        if (res.ok) return res.json();
        throw new Error("Failed to configure folder");
      })
      .then(data => {
        // Save returned folder details
        setFolderDetails({
          id: data.folder_id,
          name: data.folder_name,
          file_count: data.file_count
        });
        
        // Update user config state
        setUserConfig(prev => ({
          ...prev,
          gdriveNotesFolderId: data.folder_id
        }));

        setIsEditingFolder(false);
        setSelectedFolderPreview(null);
        setIsLoading(false);

        // Immediately start checking sync status!
        setSyncStatus({ active: true, current: 0, total: 0, error: '' });
        setTimeout(() => {
          fetch('/api/sync/status')
            .then(res => res.json())
            .then(data => {
              setSyncStatus(data || { active: true, current: 0, total: 0, error: '' });
            })
            .catch(err => console.error(err));
        }, 200);
      })
      .catch(err => {
        console.error(err);
        alert('Failed to configure folder.');
        setIsLoading(false);
      });
  };

  const handleOpenFolderModal = () => {
    setIsFolderModalOpen(true);
    setIsModalLoading(true);
    fetch('/api/gdrive/folders')
      .then(res => {
        if (!res.ok) throw new Error("Failed to fetch folders");
        return res.json();
      })
      .then(data => {
        setGdriveFolders(data || []);
        setIsModalLoading(false);
      })
      .catch(err => {
        console.error(err);
        alert("Could not load folders. Ensure Google Drive is linked and active.");
        setIsModalLoading(false);
        setIsFolderModalOpen(false);
      });
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
        <section className="glass-container" style={{ padding: '24px', marginBottom: '40px', borderRadius: '16px', background: 'linear-gradient(135deg, rgba(17, 21, 28, 0.75), rgba(56, 189, 248, 0.02))' }}>
          {!isEditingFolder && folderDetails ? (
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                <span style={{ fontSize: '2.2rem' }}>📁</span>
                <div>
                  <h3 style={{ fontWeight: 600, color: 'var(--text-primary)', display: 'flex', alignItems: 'center', gap: '10px' }}>
                    Connected Notes Directory
                    <span 
                      style={{ 
                        fontSize: '0.75rem', 
                        padding: '4px 8px', 
                        borderRadius: '9999px', 
                        background: 'rgba(16, 185, 129, 0.15)', 
                        color: 'var(--success)', 
                        fontWeight: 600,
                        border: '1px solid rgba(16, 185, 129, 0.2)'
                      }}
                    >
                      Active Sync
                    </span>
                  </h3>
                  <p style={{ color: 'var(--text-secondary)', marginTop: '4px', fontSize: '0.95rem' }}>
                    <strong style={{ color: '#fff' }}>{folderDetails.name}</strong> 
                    <span style={{ color: 'var(--text-muted)', margin: '0 8px' }}>|</span> 
                    ID: <code style={{ color: 'var(--accent)', backgroundColor: 'rgba(56, 189, 248, 0.06)', padding: '2px 6px', borderRadius: '4px' }}>{folderDetails.id}</code>
                  </p>
                </div>
              </div>
              
              <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                <div style={{ textAlign: 'right' }}>
                  <span style={{ 
                    fontSize: '0.85rem', 
                    fontWeight: 600, 
                    color: 'var(--text-muted)', 
                    textTransform: 'uppercase', 
                    letterSpacing: '0.05em' 
                  }}>
                    Folder Contents
                  </span>
                  <div style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--accent)', marginTop: '2px' }}>
                    {isDetailsLoading ? 'Loading...' : `${folderDetails.file_count} Files`}
                  </div>
                </div>
                
                <button
                  type="button"
                  onClick={() => setIsEditingFolder(true)}
                  className="btn btn-secondary"
                  style={{ borderRadius: '10px', padding: '10px 16px', display: 'flex', alignItems: 'center', gap: '6px' }}
                >
                  ✏️ Change Folder
                </button>
              </div>
            </div>
          ) : (
            <div>
              <h3 style={{ marginBottom: '6px', fontWeight: 600 }}>Configure Notebooks Location</h3>
              <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem', marginBottom: '16px' }}>
                Enter the Google Drive folder ID or click "Browse Folders" to select the folder containing your handwritten notes.
              </p>
              
              <form onSubmit={handleSaveFolder} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                <div style={{ display: 'flex', gap: '12px', maxWidth: '800px', flexWrap: 'wrap' }}>
                  <input
                    type="text"
                    placeholder="Enter Google Drive Folder ID containing your Supernote .note files"
                    value={folderIdInput}
                    onChange={(e) => {
                      const val = e.target.value;
                      setFolderIdInput(val);
                      if (val.trim() === '') {
                        setSelectedFolderPreview(null);
                      } else {
                        setSelectedFolderPreview({
                          id: val,
                          name: "Manual Input ID"
                        });
                      }
                    }}
                    style={{
                      flex: 1,
                      minWidth: '280px',
                      padding: '10px 16px',
                      borderRadius: '8px',
                      border: '1px solid var(--border-frosted)',
                      backgroundColor: 'rgba(255, 255, 255, 0.05)',
                      color: '#fff',
                      outline: 'none',
                      fontFamily: 'var(--font-body)'
                    }}
                  />
                  <button
                    type="button"
                    onClick={handleOpenFolderModal}
                    className="btn"
                    style={{
                      backgroundColor: 'rgba(255, 255, 255, 0.05)',
                      color: 'var(--text-primary)',
                      border: '1px solid var(--border-frosted)',
                      borderRadius: '8px',
                      cursor: 'pointer',
                      padding: '10px 16px'
                    }}
                  >
                    📁 Browse Folders
                  </button>
                </div>

                {selectedFolderPreview && (
                  <div style={{ 
                    padding: '12px 16px', 
                    borderRadius: '8px', 
                    backgroundColor: 'rgba(56, 189, 248, 0.05)', 
                    border: '1px solid rgba(56, 189, 248, 0.15)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    maxWidth: '800px'
                  }}>
                    <div>
                      <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Selected Folder Preview:</span>
                      <div style={{ fontWeight: 600, color: 'var(--accent)', marginTop: '2px' }}>
                        {selectedFolderPreview.name} <span style={{ fontWeight: 400, color: 'var(--text-muted)', fontSize: '0.85rem' }}>(ID: {selectedFolderPreview.id})</span>
                      </div>
                    </div>
                  </div>
                )}

                <div style={{ display: 'flex', gap: '12px' }}>
                  <button type="submit" className="btn btn-primary" disabled={!folderIdInput}>
                    💾 Save Config
                  </button>
                  
                  {userConfig.gdriveNotesFolderId && (
                    <button
                      type="button"
                      onClick={() => {
                        setIsEditingFolder(false);
                        setFolderIdInput(userConfig.gdriveNotesFolderId);
                        setSelectedFolderPreview(null);
                      }}
                      className="btn btn-secondary"
                    >
                      Cancel
                    </button>
                  )}
                </div>
              </form>
            </div>
          )}
        </section>
      )}

      {/* Real-time Google Drive Syncing Progress Card */}
      {syncStatus.active && (
        <div 
          className="glass-container sync-pulse" 
          style={{ 
            padding: '20px', 
            marginBottom: '40px', 
            borderRadius: '16px',
            border: '1px solid rgba(56, 189, 248, 0.3)',
            display: 'flex',
            flexDirection: 'column',
            gap: '12px',
            background: 'linear-gradient(135deg, rgba(17, 21, 28, 0.85), rgba(56, 189, 248, 0.05))',
            transition: 'var(--transition-smooth)'
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <span className="spin-slow" style={{ fontSize: '1.25rem', display: 'inline-block' }}>🔄</span>
              <h4 style={{ fontWeight: 600, color: 'var(--text-primary)' }}>Synchronizing GDrive Notebooks...</h4>
            </div>
            <span style={{ fontSize: '0.875rem', fontWeight: 600, color: 'var(--accent)' }}>
              {syncStatus.total > 0 ? `${Math.round((syncStatus.current / syncStatus.total) * 100)}%` : 'Initializing...'}
            </span>
          </div>

          <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', margin: 0 }}>
            {syncStatus.total > 0 
              ? `Processing page files: ${syncStatus.current} of ${syncStatus.total} downloaded.`
              : 'Listing Google Drive directory and resolving files...'
            }
          </p>

          {/* Progress bar wrapper */}
          <div style={{ 
            width: '100%', 
            height: '8px', 
            backgroundColor: 'rgba(255, 255, 255, 0.05)', 
            borderRadius: '9999px',
            overflow: 'hidden',
            border: '1px solid var(--border-frosted)'
          }}>
            <div style={{ 
              width: `${syncStatus.total > 0 ? (syncStatus.current / syncStatus.total) * 100 : 0}%`, 
              height: '100%', 
              background: 'linear-gradient(90deg, var(--accent), #0ea5e9)',
              borderRadius: '9999px',
              transition: 'width 0.3s cubic-bezier(0.4, 0, 0.2, 1)'
            }} />
          </div>
        </div>
      )}

      {syncStatus.error && (
        <div 
          className="glass-container" 
          style={{ 
            padding: '20px', 
            marginBottom: '40px', 
            borderRadius: '16px',
            border: '1px solid rgba(239, 68, 68, 0.3)',
            background: 'linear-gradient(135deg, rgba(17, 21, 28, 0.85), rgba(239, 68, 68, 0.05))',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '16px'
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <span style={{ fontSize: '1.5rem', color: 'var(--danger)' }}>⚠️</span>
            <div>
              <h4 style={{ fontWeight: 600, color: 'var(--text-primary)', marginBottom: '4px' }}>Sync Failed</h4>
              <p style={{ fontSize: '0.875rem', color: '#fca5a5', margin: 0 }}>{syncStatus.error}</p>
            </div>
          </div>
          <button 
            className="btn"
            onClick={() => setSyncStatus(prev => ({ ...prev, error: '' }))}
            style={{
              padding: '6px 12px',
              backgroundColor: 'rgba(255, 255, 255, 0.05)',
              border: '1px solid var(--border-frosted)',
              borderRadius: '8px',
              fontSize: '0.8125rem',
              color: 'var(--text-primary)',
              cursor: 'pointer'
            }}
          >
            Dismiss
          </button>
        </div>
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

      {/* Google Drive Folder Selector Modal */}
      {isFolderModalOpen && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.6)',
          backdropFilter: 'blur(12px)',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          zIndex: 1000
        }}>
          <div className="glass-container" style={{
            maxWidth: '500px',
            width: '90%',
            maxHeight: '80vh',
            padding: '30px',
            borderRadius: '24px',
            display: 'flex',
            flexDirection: 'column',
            gap: '20px',
            boxShadow: '0 20px 50px rgba(0, 0, 0, 0.4)',
            border: '1px solid var(--border-frosted)'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <h3 style={{ fontSize: '1.25rem', fontWeight: 600, margin: 0 }}>📁 Select Notes Folder</h3>
              <button
                onClick={() => setIsFolderModalOpen(false)}
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

            <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem', margin: 0 }}>
              Select a Google Drive folder containing your Supernote .note files.
            </p>

            {/* Search Input */}
            <input
              type="text"
              placeholder="🔍 Search folders..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{
                width: '100%',
                padding: '10px 16px',
                borderRadius: '10px',
                border: '1px solid var(--border-frosted)',
                backgroundColor: 'rgba(255, 255, 255, 0.05)',
                color: '#fff',
                outline: 'none',
                fontFamily: 'var(--font-body)'
              }}
            />

            {/* Folder List container */}
            <div style={{
              flex: 1,
              overflowY: 'auto',
              display: 'flex',
              flexDirection: 'column',
              gap: '8px',
              minHeight: '200px',
              paddingRight: '4px'
            }}>
              {isModalLoading ? (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1, color: 'var(--text-secondary)' }}>
                  Loading GDrive folders...
                </div>
              ) : gdriveFolders.filter(f => f.name.toLowerCase().includes(searchQuery.toLowerCase())).length === 0 ? (
                <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1, color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
                  No matching folders found.
                </div>
              ) : (
                gdriveFolders
                  .filter(f => f.name.toLowerCase().includes(searchQuery.toLowerCase()))
                  .map(folder => (
                    <div
                      key={folder.id}
                      onClick={() => {
                        setSelectedFolderPreview({
                          id: folder.id,
                          name: folder.name
                        });
                        setFolderIdInput(folder.id);
                        setIsFolderModalOpen(false);
                      }}
                      className="glass-container"
                      style={{
                        padding: '12px 16px',
                        borderRadius: '10px',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '12px',
                        border: '1px solid transparent',
                        transition: 'var(--transition-smooth)'
                      }}
                      onMouseOver={(e) => {
                        e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, 0.08)';
                        e.currentTarget.style.borderColor = 'var(--accent-glow)';
                      }}
                      onMouseOut={(e) => {
                        e.currentTarget.style.backgroundColor = 'transparent';
                        e.currentTarget.style.borderColor = 'transparent';
                      }}
                    >
                      <span style={{ fontSize: '1.25rem' }}>📁</span>
                      <div style={{ display: 'flex', flexDirection: 'column' }}>
                        <span style={{ fontWeight: 500, fontSize: '0.95rem' }}>{folder.name}</span>
                        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>ID: {folder.id}</span>
                      </div>
                    </div>
                  ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
