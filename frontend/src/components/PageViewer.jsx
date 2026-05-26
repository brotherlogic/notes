import React, { useState, useEffect } from 'react';

export default function PageViewer({ notebook, onClose }) {
  const [currentPageIndex, setCurrentPageIndex] = useState(0);
  const [showProcessed, setShowProcessed] = useState(false);
  const [animate, setAnimate] = useState(false);

  // Filter pages based on whether we are showing processed ones or not
  const filteredPages = (notebook.pages || []).filter(page => {
    if (showProcessed) return true;
    return !page.processed;
  });

  const currentPage = filteredPages[currentPageIndex];

  // Trigger page flip animations
  useEffect(() => {
    setAnimate(true);
    const timer = setTimeout(() => setAnimate(false), 300);
    return () => clearTimeout(timer);
  }, [currentPageIndex]);

  const handleNextPage = () => {
    if (currentPageIndex < filteredPages.length - 1) {
      setCurrentPageIndex(currentPageIndex + 1);
    }
  };

  const handlePrevPage = () => {
    if (currentPageIndex > 0) {
      setCurrentPageIndex(currentPageIndex - 1);
    }
  };

  const handleMarkProcessed = () => {
    if (!currentPage) return;
    fetch(`/api/pages/${currentPage.id}/processed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ processed: !currentPage.processed })
    })
      .then(res => {
        if (res.ok) {
          currentPage.processed = !currentPage.processed;
          // Trigger next page index bounds check
          if (currentPageIndex >= filteredPages.length - 1 && currentPageIndex > 0) {
            setCurrentPageIndex(currentPageIndex - 1);
          } else {
            setCurrentPageIndex(currentPageIndex); // force re-render
          }
        }
      })
      .catch(err => console.error(err));
  };

  return (
    <div style={{ padding: '24px', maxWidth: '900px', margin: '0 auto' }}>
      {/* Navigation Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
        <button onClick={onClose} className="btn btn-secondary">
          ← Back to Notebooks
        </button>

        <h2 style={{ fontSize: '1.5rem', fontWeight: 600 }}>{notebook.title}</h2>

        {/* Processed Toggles */}
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
              position: 'relative'
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
                Page {currentPage.pageNumber || currentPageIndex + 1} of {filteredPages.length}
              </span>
              
              <div style={{ display: 'flex', gap: '10px' }}>
                <button
                  onClick={handleMarkProcessed}
                  className="btn btn-secondary"
                  style={{
                    padding: '6px 12px',
                    fontSize: '0.875rem',
                    borderColor: currentPage.processed ? 'var(--success)' : 'var(--border-frosted)',
                    color: currentPage.processed ? 'var(--success)' : 'var(--text-primary)'
                  }}
                >
                  {currentPage.processed ? '✓ Processed' : 'Mark Processed'}
                </button>
              </div>
            </div>

            {/* Note Page Image Sheet */}
            <div style={{
              position: 'relative',
              width: '100%',
              maxHeight: '600px',
              overflow: 'hidden',
              borderRadius: '12px',
              backgroundColor: 'rgba(0, 0, 0, 0.2)',
              display: 'flex',
              justifyContent: 'center',
              border: '1px solid var(--border-frosted)'
            }}>
              <img
                src={`/api/pages/${currentPage.id}/image`}
                alt={`Notebook Page ${currentPage.pageNumber}`}
                style={{
                  maxWidth: '100%',
                  height: 'auto',
                  objectFit: 'contain',
                  opacity: currentPage.processed ? 0.5 : 1,
                  transition: 'var(--transition-smooth)'
                }}
              />
              
              {/* Coming Soon Crop Overlay guidance */}
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
                ℹ️ Drag to Crop (Coming in Issue 5)
              </div>
            </div>

            {/* Pagination Controls */}
            <div style={{ display: 'flex', gap: '16px', marginTop: '24px' }}>
              <button
                onClick={handlePrevPage}
                disabled={currentPageIndex === 0}
                className="btn btn-secondary"
                style={{ opacity: currentPageIndex === 0 ? 0.4 : 1 }}
              >
                ◀ Previous Page
              </button>
              <button
                onClick={handleNextPage}
                disabled={currentPageIndex === filteredPages.length - 1}
                className="btn btn-secondary"
                style={{ opacity: currentPageIndex === filteredPages.length - 1 ? 0.4 : 1 }}
              >
                Next Page ▶
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
