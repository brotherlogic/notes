package sync_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"

	"github.com/brotherlogic/notes/internal/sync"
)

// helper to create a valid minimal PNG byte slice
func createValidPNGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestConvertNoteToPNGs_EmptyData(t *testing.T) {
	ctx := context.Background()
	_, err := sync.ConvertNoteToPNGs(ctx, "TestNote", nil)
	if err == nil {
		t.Fatal("expected error for nil data, got nil")
	}
	if !errors.Is(err, sync.ErrCorruptBinary) {
		t.Errorf("expected error to wrap ErrCorruptBinary, got %v", err)
	}

	_, err = sync.ConvertNoteToPNGs(ctx, "TestNote", []byte{})
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
	if !errors.Is(err, sync.ErrCorruptBinary) {
		t.Errorf("expected error to wrap ErrCorruptBinary, got %v", err)
	}
}

func TestConvertNoteToPNGs_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sync.ConvertNoteToPNGs(ctx, "TestNote", []byte("pages=2"))
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestConvertNoteToPNGs_MockFormat(t *testing.T) {
	ctx := context.Background()
	data := []byte("pages=3")

	pngs, err := sync.ConvertNoteToPNGs(ctx, "LectureNote", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pngs) != 3 {
		t.Fatalf("expected 3 mock pages, got %d", len(pngs))
	}

	// Validate each generated page is a valid PNG with 800x1000 dimensions
	for i, pngBytes := range pngs {
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("page %d: failed to decode generated mock PNG: %v", i+1, err)
		}
		bounds := img.Bounds()
		if bounds.Dx() != 800 || bounds.Dy() != 1000 {
			t.Errorf("page %d: expected size 800x1000, got %dx%d", i+1, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestConvertNoteToPNGs_InvalidMockPageCounts(t *testing.T) {
	ctx := context.Background()

	// Zero pages
	_, err := sync.ConvertNoteToPNGs(ctx, "TestNote", []byte("pages=0"))
	if err == nil {
		t.Fatal("expected error for pages=0, got nil")
	}
	if !errors.Is(err, sync.ErrUnsupportedStructure) {
		t.Errorf("expected error to wrap ErrUnsupportedStructure, got %v", err)
	}

	// Negative pages
	_, err = sync.ConvertNoteToPNGs(ctx, "TestNote", []byte("pages=-5"))
	if err == nil {
		t.Fatal("expected error for negative page count, got nil")
	}
	if !errors.Is(err, sync.ErrUnsupportedStructure) {
		t.Errorf("expected error to wrap ErrUnsupportedStructure, got %v", err)
	}

	// Excessively large page count
	_, err = sync.ConvertNoteToPNGs(ctx, "TestNote", []byte("pages=600"))
	if err == nil {
		t.Fatal("expected error for excessive page count, got nil")
	}
	if !errors.Is(err, sync.ErrUnsupportedStructure) {
		t.Errorf("expected error to wrap ErrUnsupportedStructure, got %v", err)
	}
}

func TestConvertNoteToPNGs_UnsupportedStructure(t *testing.T) {
	ctx := context.Background()
	// Plain binary/text that matches neither mock syntax nor PNG headers
	garbage := []byte("this is random non-matching data that should be rejected")

	_, err := sync.ConvertNoteToPNGs(ctx, "TestNote", garbage)
	if err == nil {
		t.Fatal("expected error for unsupported structure, got nil")
	}
	if !errors.Is(err, sync.ErrUnsupportedStructure) {
		t.Errorf("expected error to wrap ErrUnsupportedStructure, got %v", err)
	}
}

func TestConvertNoteToPNGs_ValidBinaryPNGs(t *testing.T) {
	ctx := context.Background()

	// Build a mock Supernote file containing two valid embedded PNGs
	png1 := createValidPNGBytes()
	png2 := createValidPNGBytes()

	var binaryData bytes.Buffer
	binaryData.WriteString("some_prefix_headers_before_first_png")
	binaryData.Write(png1)
	binaryData.WriteString("some_headers_between_pngs")
	binaryData.Write(png2)
	binaryData.WriteString("some_suffix_bytes")

	pngs, err := sync.ConvertNoteToPNGs(ctx, "BinaryNote", binaryData.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pngs) != 2 {
		t.Fatalf("expected 2 extracted PNGs, got %d", len(pngs))
	}

	// Verify both match the generated mock PNG structures
	for i, pngBytes := range pngs {
		_, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("extracted page %d is not a valid PNG: %v", i+1, err)
		}
	}
}

func TestConvertNoteToPNGs_CorruptBinaryPNGs(t *testing.T) {
	ctx := context.Background()

	// 1. Contains PNG header but no IEND chunk
	corrupt1 := []byte("\x89PNG\r\n\x1a\n some corrupt bytes that never end")
	_, err := sync.ConvertNoteToPNGs(ctx, "CorruptNote", corrupt1)
	if err == nil {
		t.Fatal("expected error for missing IEND, got nil")
	}
	if !errors.Is(err, sync.ErrCorruptBinary) {
		t.Errorf("expected error to wrap ErrCorruptBinary, got %v", err)
	}

	// 2. Contains PNG header and IEND but actual PNG contents are completely invalid/corrupt
	var corrupt2 bytes.Buffer
	corrupt2.Write([]byte("\x89PNG\r\n\x1a\n"))
	corrupt2.Write([]byte("nonsense bytes of length greater than fifty to trigger extraction"))
	corrupt2.Write([]byte("IEND\x00\x00\x00\x00"))

	_, err = sync.ConvertNoteToPNGs(ctx, "CorruptNote", corrupt2.Bytes())
	if err == nil {
		t.Fatal("expected error for invalid PNG data, got nil")
	}
	if !errors.Is(err, sync.ErrCorruptBinary) {
		t.Errorf("expected error to wrap ErrCorruptBinary, got %v", err)
	}
}

func TestGenerateMockPage(t *testing.T) {
	// Directly test GenerateMockPage for premium visual and size correctness
	imgBytes, err := sync.GenerateMockPage("ExtremelyLongNotebookTitleThatNeedsTruncation", 1)
	if err != nil {
		t.Fatalf("GenerateMockPage failed: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		t.Fatalf("Failed to decode mock page PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 800 || bounds.Dy() != 1000 {
		t.Errorf("Expected 800x1000 page, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
