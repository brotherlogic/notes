import React, { useState, useEffect, useRef } from 'react';
import GitHubRepoModal from './GitHubRepoModal';

export default function PageViewer({ notebook, onClose, onUpdateNotebook }) {
  const [pages, setPages] = useState(() => notebook.pages || []);
  const [currentPageIndex, setCurrentPageIndex] = useState(0);
  const [showProcessed, setShowProcessed] = useState(false);
  const [animate, setAnimate] = useState(false);
  const [isRepoModalOpen, setIsRepoModalOpen] = useState(false);

  const isArchived = notebook.status === 'NOTEBOOK_DELETED_ON_REMOTE';
  const isUnprocessable = notebook.status === 'NOTEBOOK_UNPROCESSABLE';
  const linkedRepo = notebook.githubProject || notebook.github_project || '';

  // Crop State
  const [isDrawing, setIsDrawing] = useState(false);
  const [startCoords, setStartCoords] = useState({ x: 0, y: 0 });
  const [cropBox, setCropBox] = useState(null); // { x, y, w, h } in display pixels
  const [showFormModal, setShowFormModal] = useState(false);
  
  // Issue Form State
  const [issueTitle, setIssueTitle] = useState('');
  const [issueBody, setIssueBody] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const imgRef = useRef(null);
  const containerRef = useRef(null);

  useEffect(() => {
    setPages(notebook.pages || []);
  }, [notebook.pages]);

  const pagesWithIndex = pages.map((page, index) => ({ page, index }));
  const filteredPages = pagesWithIndex
    .filter(({ page }) => {
      if (showProcessed) return true;
      return !page.processed;
    })
    .sort((a, b) => {
      if (a.page.processed && !b.page.processed) return 1;
      if (!a.page.processed && b.page.processed) return -1;
      return a.index - b.index;
    })
    .map(({ page }) => page);

  const safePageIndex = filteredPages.length > 0
    ? Math.min(currentPageIndex, filteredPages.length - 1)
    : 0;
  const currentPage = filteredPages[safePageIndex];

  // Trigger page flip animations and reset crop box
  useEffect(() => {
    setAnimate(true);
    setCropBox(null);
    const timer = setTimeout(() => setAnimate(false), 300);
    return () => clearTimeout(timer);
  }, [safePageIndex, currentPage?.id]);

  const handleNextPage = () => {
    if (safePageIndex < filteredPages.length - 1) {
      setCurrentPageIndex(safePageIndex + 1);
    }
  };

  const handlePrevPage = () => {
    if (safePageIndex > 0) {
      setCurrentPageIndex(safePageIndex - 1);
    }
  };

  const handleMarkProcessed = () => {
    if (!currentPage || isArchived) return;
    const pageId = currentPage.id;
    const nextProcessed = !currentPage.processed;

    fetch(`/api/pages/${pageId}/processed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ processed: nextProcessed })
    })
      .then(res => {
        if (res.ok) {
          const updatedPages = pages.map(page =>
            page.id === pageId ? { ...page, processed: nextProcessed } : page
          );
          if (safePageIndex >= filteredPages.length - 1 && safePageIndex > 0) {
            setCurrentPageIndex(safePageIndex - 1);
          }
          setPages(updatedPages);
          if (onUpdateNotebook) {
            onUpdateNotebook({ ...notebook, pages: updatedPages });
          }
        }
      })
      .catch(err => console.error(err));
  };

  // Click & Drag Crop Drawing Handlers
  const handleMouseDown = (e) => {
    if (!containerRef.current || currentPage?.processed || isArchived) return;
    const rect = containerRef.current.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    setIsDrawing(true);
    setStartCoords({ x, y });
    setCropBox({ x, y, w: 0, h: 0 });
  };

  const handleMouseMove = (e) => {
    if (!isDrawing || !containerRef.current || !cropBox || isArchived) return;
    const rect = containerRef.current.getBoundingClientRect();
    const currentX = e.clientX - rect.left;
    const currentY = e.clientY - rect.top;

    const x = Math.min(startCoords.x, currentX);
    const y = Math.min(startCoords.y, currentY);
    const w = Math.abs(startCoords.x - currentX);
    const h = Math.abs(startCoords.y - currentY);

    setCropBox({ x, y, w, h });
  };

  const handleMouseUp = () => {
    if (!isDrawing || isArchived) return;
    setIsDrawing(false);
    if (cropBox && cropBox.w > 10 && cropBox.h > 10) {
      // Show form modal to file issue
      setShowFormModal(true);
    } else {
      setCropBox(null);
    }
  };

  const handleSubmitIssue = (e) => {
    e.preventDefault();
    if (!imgRef.current || !cropBox || isArchived) return;

    setIsSubmitting(true);

    // Calculate scale factor relative to natural image dimensions
    const img = imgRef.current;
    const scaleX = img.naturalWidth / img.clientWidth;
    const scaleY = img.naturalHeight / img.clientHeight;

    const payload = {
      page_id: currentPage.id,
      x: cropBox.x * scaleX,
      y: cropBox.y * scaleY,
      width: cropBox.w * scaleX,
      height: cropBox.h * scaleY,
      title: issueTitle,
      body: issueBody
    };

    fetch('/api/issues/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
      .then(res => {
        if (res.ok) {
          alert('GitHub issue created successfully!');
          setCropBox(null);
          setIssueTitle('');
          setIssueBody('');
          setShowFormModal(false);
        } else {
          alert('Failed to create GitHub issue.');
        }
        setIsSubmitting(false);
      })
      .catch(err => {
        console.error(err);
        setIsSubmitting(false);
      });
  };

  return (
    <div style={{ padding: '24px', maxWidth: '900px', margin: '0 auto' }}>
      {/* Navigation Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px', flexWrap: 'wrap', gap: '12px' }}>
        <button onClick={onClose} className="btn btn-secondary">
          ← Back to Notebooks
        </button>

        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '6px' }}>
          <h2 style={{ fontSize: '1.5rem', fontWeight: 600 }}>{notebook.title}</h2>
          <button
            type="button"
            disabled={isArchived}
            onClick={() => !isArchived && setIsRepoModalOpen(true)}
            style={{
              padding: '4px 10px',
              borderRadius: '8px',
              fontSize: '0.75rem',
              fontWeight: 500,
              backgroundColor: linkedRepo ? 'rgba(56, 189, 248, 0.1)' : 'rgba(255, 255, 255, 0.04)',
              color: linkedRepo ? 'var(--accent)' : 'var(--text-secondary)',
              border: linkedRepo ? '1px solid rgba(56, 189, 248, 0.25)' : '1px solid var(--border-frosted)',
              cursor: isArchived ? 'not-allowed' : 'pointer',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              transition: 'var(--transition-smooth)',
              fontFamily: 'var(--font-body)'
            }}
          >
            <span>🐙</span>
            <span>{linkedRepo ? linkedRepo : 'Link GitHub Repo'}</span>
            {!isArchived && <span style={{ opacity: 0.7, fontSize: '0.7rem' }}>✏️</span>}
          </button>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
            <input
              type="checkbox"
              checked={showProcessed}
              onChange={(e) => {
                setShowProcessed(e.target.checked);
                setCurrentPageIndex(0);
              }}
              style={{ cursor: 'pointer' }}
            />
            Show Processed Pages
          </label>
        </div>
      </div>

      {/* Status Banners */}
      {isArchived && (
        <div 
          className="glass-container" 
          style={{ 
            padding: '16px 20px', 
            marginBottom: '24px', 
            borderRadius: '16px',
            border: '1px solid rgba(148, 163, 184, 0.3)',
            background: 'linear-gradient(135deg, rgba(17, 21, 28, 0.85), rgba(148, 163, 184, 0.05))',
            display: 'flex',
            alignItems: 'center',
            gap: '16px'
          }}
        >
          <span style={{ fontSize: '1.75rem' }}>📁</span>
          <div>
            <h4 style={{ fontWeight: 600, color: 'var(--text-primary)', fontSize: '1.05rem', marginBottom: '2px' }}>Archived Notebook</h4>
            <p style={{ fontSize: '0.9rem', color: 'var(--text-secondary)', margin: 0 }}>
              This notebook has been deleted from Google Drive and is read-only.
            </p>
          </div>
        </div>
      )}

      {isUnprocessable && (
        <div 
          className="glass-container" 
          style={{ 
            padding: '16px 20px', 
            marginBottom: '24px', 
            borderRadius: '16px',
            border: '1px solid rgba(239, 68, 68, 0.4)',
            background: 'linear-gradient(135deg, rgba(17, 21, 28, 0.85), rgba(239, 68, 68, 0.05))',
            display: 'flex',
            alignItems: 'center',
            gap: '16px'
          }}
        >
          <span style={{ fontSize: '1.75rem' }}>⚠️</span>
          <div>
            <h4 style={{ fontWeight: 600, color: 'var(--text-primary)', fontSize: '1.05rem', marginBottom: '2px' }}>Corrupted or Unprocessable</h4>
            <p style={{ fontSize: '0.9rem', color: '#fca5a5', margin: 0 }}>
              This notebook could not be processed. Please check if the file is corrupted.
            </p>
          </div>
        </div>
      )}

      {filteredPages.length === 0 ? (
        <div className="glass-container" style={{ padding: '60px', textAlign: 'center', color: 'var(--text-secondary)', borderRadius: '16px' }}>
          No active pages in this notebook. Mark pages as active or enable "Show Processed Pages".
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* Main Visual Frosted Glass Sheet */}
          <div
            className={`glass-container ${animate ? 'page-flip-effect' : ''}`}
            style={{
              padding: '24px',
              borderRadius: '20px',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              position: 'relative',
              opacity: currentPage?.processed ? 0.65 : 1,
              transition: 'var(--transition-smooth)'
            }}
          >
            {/* Page Metadata Overlay */}
            <div style={{
              width: '100%',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: '16px',
              borderBottom: '1px solid var(--border-frosted)',
              paddingBottom: '12px'
            }}>
              <span style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', fontWeight: 500 }}>
                Page {currentPage.pageNumber || safePageIndex + 1} of {filteredPages.length}
              </span>
              
              <div style={{ display: 'flex', gap: '10px' }}>
                <button
                  onClick={handleMarkProcessed}
                  disabled={isArchived}
                  className="btn btn-secondary"
                  style={{
                    padding: '6px 12px',
                    fontSize: '0.875rem',
                    borderColor: currentPage.processed ? 'var(--success)' : 'var(--border-frosted)',
                    color: currentPage.processed ? 'var(--success)' : 'var(--text-primary)',
                    opacity: isArchived ? 0.5 : 1,
                    cursor: isArchived ? 'not-allowed' : 'pointer'
                  }}
                >
                  {currentPage.processed ? '✓ Processed' : 'Mark Processed'}
                </button>
              </div>
            </div>

            {/* Interactive Image Drawing Container */}
            <div
              ref={containerRef}
              onMouseDown={handleMouseDown}
              onMouseMove={handleMouseMove}
              onMouseUp={handleMouseUp}
              style={{
                position: 'relative',
                width: '100%',
                maxHeight: '600px',
                overflow: 'hidden',
                borderRadius: '12px',
                backgroundColor: 'rgba(0, 0, 0, 0.2)',
                display: 'flex',
                justifyContent: 'center',
                border: '1px solid var(--border-frosted)',
                cursor: (currentPage.processed || isArchived) ? 'not-allowed' : 'crosshair',
                userSelect: 'none'
              }}
            >
              <img
                ref={imgRef}
                src={`/api/pages/${currentPage.id}/image`}
                alt={`Notebook Page ${currentPage.pageNumber}`}
                draggable={false}
                style={{
                  maxWidth: '100%',
                  height: 'auto',
                  objectFit: 'contain',
                  opacity: currentPage.processed ? 0.5 : 1,
                  transition: 'var(--transition-smooth)'
                }}
              />

              {/* Crop Bounding Box Overlay */}
              {cropBox && (
                <div style={{
                  position: 'absolute',
                  left: `${cropBox.x}px`,
                  top: `${cropBox.y}px`,
                  width: `${cropBox.w}px`,
                  height: `${cropBox.h}px`,
                  border: '2px solid var(--accent)',
                  backgroundColor: 'rgba(56, 189, 248, 0.15)',
                  boxShadow: '0 0 8px rgba(56, 189, 248, 0.4)',
                  pointerEvents: 'none'
                }} />
              )}
              
              {!currentPage.processed && !isArchived && (
                <div style={{
                  position: 'absolute',
                  top: '12px',
                  right: '12px',
                  background: 'rgba(0,0,0,0.6)',
                  padding: '4px 8px',
                  borderRadius: '6px',
                  fontSize: '0.75rem',
                  color: 'var(--text-secondary)'
                }}>
                  🖱️ Click & Drag to Crop
                </div>
              )}

              {isArchived && (
                <div style={{
                  position: 'absolute',
                  top: '12px',
                  right: '12px',
                  background: 'rgba(0,0,0,0.6)',
                  padding: '4px 8px',
                  borderRadius: '6px',
                  fontSize: '0.75rem',
                  color: 'var(--danger)'
                }}>
                  🔒 Read-Only
                </div>
              )}
            </div>

            {/* Pagination Controls */}
            <div style={{ display: 'flex', gap: '16px', marginTop: '24px' }}>
              <button
                onClick={handlePrevPage}
                disabled={safePageIndex === 0}
                className="btn btn-secondary"
                style={{ opacity: safePageIndex === 0 ? 0.4 : 1 }}
              >
                ◀ Previous Page
              </button>
              <button
                onClick={handleNextPage}
                disabled={safePageIndex === filteredPages.length - 1}
                className="btn btn-secondary"
                style={{ opacity: safePageIndex === filteredPages.length - 1 ? 0.4 : 1 }}
              >
                Next Page ▶
              </button>
            </div>
          </div>
        </div>
      )}

      {/* GitHub Issue Creation Glass Modal Form */}
      {showFormModal && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.65)',
          backdropFilter: 'blur(4px)',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          zIndex: 1000
        }}>
          <div
            className="glass-container"
            style={{
              maxWidth: '480px',
              width: '100%',
              padding: '32px',
              borderRadius: '20px',
              margin: '20px'
            }}
          >
            <h3 style={{ fontSize: '1.5rem', fontWeight: 600, marginBottom: '12px' }}>
              Create GitHub Issue from Crop
            </h3>

            <div style={{
              marginBottom: '16px',
              padding: '10px 12px',
              borderRadius: '10px',
              backgroundColor: linkedRepo ? 'rgba(56, 189, 248, 0.08)' : 'rgba(239, 68, 68, 0.1)',
              border: linkedRepo ? '1px solid rgba(56, 189, 248, 0.2)' : '1px solid rgba(239, 68, 68, 0.3)',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              gap: '10px',
              fontSize: '0.85rem'
            }}>
              <span style={{ color: linkedRepo ? 'var(--text-primary)' : '#fca5a5' }}>
                {linkedRepo ? (
                  <>Target Repo: <strong style={{ color: 'var(--accent)' }}>{linkedRepo}</strong></>
                ) : (
                  '⚠️ No GitHub repository linked to this notebook.'
                )}
              </span>
              <button
                type="button"
                onClick={() => setIsRepoModalOpen(true)}
                className="btn btn-secondary"
                style={{ padding: '4px 10px', fontSize: '0.75rem' }}
              >
                {linkedRepo ? 'Change' : 'Link Repo'}
              </button>
            </div>
            
            <form onSubmit={handleSubmitIssue} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <label style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>Issue Title</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Fix button alignment in login page"
                  value={issueTitle}
                  onChange={(e) => setIssueTitle(e.target.value)}
                  style={{
                    padding: '10px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border-frosted)',
                    backgroundColor: 'rgba(255, 255, 255, 0.05)',
                    color: '#fff',
                    outline: 'none'
                  }}
                />
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <label style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>Description</label>
                <textarea
                  rows={4}
                  placeholder="Describe the issue. The cropped screenshot will be appended automatically."
                  value={issueBody}
                  onChange={(e) => setIssueBody(e.target.value)}
                  style={{
                    padding: '10px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border-frosted)',
                    backgroundColor: 'rgba(255, 255, 255, 0.05)',
                    color: '#fff',
                    outline: 'none',
                    resize: 'vertical',
                    fontFamily: 'var(--font-body)'
                  }}
                />
              </div>

              <div style={{ display: 'flex', gap: '12px', marginTop: '12px', justifyContent: 'flex-end' }}>
                <button
                  type="button"
                  onClick={() => {
                    setShowFormModal(false);
                    setCropBox(null);
                  }}
                  className="btn btn-secondary"
                  disabled={isSubmitting}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={isSubmitting || !linkedRepo}
                >
                  {isSubmitting ? 'Filing Issue...' : '🐙 File Issue'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* GitHub Repository Link Modal */}
      <GitHubRepoModal
        notebook={notebook}
        isOpen={isRepoModalOpen}
        onClose={() => setIsRepoModalOpen(false)}
        onSaved={(updatedNb) => {
          if (onUpdateNotebook) {
            onUpdateNotebook(updatedNb);
          }
        }}
      />
    </div>
  );
}
