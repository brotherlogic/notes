package sync_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
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

func TestConvertRealNote(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join("..", "..", "test_note", "ai cluster.note")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read real test note: %v", err)
	}

	pngs, err := sync.ConvertNoteToPNGs(ctx, "ai cluster.note", data)
	if err != nil {
		t.Fatalf("failed to convert real test note: %v", err)
	}

	t.Logf("Successfully converted real test note. Found %d pages.", len(pngs))
	if len(pngs) != 2 {
		t.Fatalf("expected 2 pages from footer table, got %d", len(pngs))
	}

	for i, imgBytes := range pngs {
		img, err := png.Decode(bytes.NewReader(imgBytes))
		if err != nil {
			t.Fatalf("page %d: failed to decode extracted PNG: %v", i+1, err)
		}
		bounds := img.Bounds()
		t.Logf("Page %d size: %dx%d", i+1, bounds.Dx(), bounds.Dy())
		if bounds.Dx() != 1404 || bounds.Dy() != 1872 {
			t.Errorf("page %d: expected native Supernote resolution 1404x1872, got %dx%d", i+1, bounds.Dx(), bounds.Dy())
		}

		// Check top-left corner is white paper background (#FFFFFF)
		r0, g0, b0, a0 := img.At(0, 0).RGBA()
		if r0 != 0xffff || g0 != 0xffff || b0 != 0xffff || a0 != 0xffff {
			t.Errorf("page %d: expected white paper background at (0,0), got RGBA(%d,%d,%d,%d)", i+1, r0>>8, g0>>8, b0>>8, a0>>8)
		}

		// Verify actual black ink strokes exist on the page
		blackPixels := 0
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r == 0 && g == 0 && b == 0 {
					blackPixels++
				}
			}
		}
		if blackPixels < 10000 {
			t.Errorf("page %d: expected >10000 decoded black stroke pixels, got %d", i+1, blackPixels)
		}
	}

	if bytes.Equal(pngs[0], pngs[1]) {
		t.Errorf("expected page 1 and page 2 to have distinct handwritten stroke renders, but PNG bytes were identical")
	}
}

func TestConvertNoteToPNGs_CorruptSupernoteBinary(t *testing.T) {
	ctx := context.Background()
	corrupt := []byte("noteSN_FILE_VER_20230015<FILE_TYPE:NOTE><FINALOPERATION_PAGE:2>corrupt_truncated")

	_, err := sync.ConvertNoteToPNGs(ctx, "CorruptRealNote", corrupt)
	if err == nil {
		t.Fatal("expected error for corrupt Supernote binary, got nil")
	}
	if !errors.Is(err, sync.ErrCorruptBinary) && !errors.Is(err, sync.ErrUnsupportedStructure) {
		t.Errorf("expected error to wrap ErrCorruptBinary or ErrUnsupportedStructure, got %v", err)
	}
}

func appendBlock(buf *bytes.Buffer, payload []byte) uint32 {
	offset := uint32(buf.Len())
	var lenBytes [4]byte
	binary.LittleEndian.PutUint32(lenBytes[:], uint32(len(payload)))
	buf.Write(lenBytes[:])
	buf.Write(payload)
	return offset
}

// buildRattaRLELayer encodes a full 1404x1872 layer where the first runLen pixels have colorCode
// and the remaining pixels are transparent (0x62).
func buildRattaRLELayer(colorCode byte, runLen int) []byte {
	total := 1404 * 1872
	var out bytes.Buffer
	writeRun := func(code byte, n int) {
		for n >= 0x4000 {
			out.WriteByte(code)
			out.WriteByte(0xff)
			n -= 0x4000
		}
		for n > 0 {
			chunk := n
			if chunk > 128 {
				chunk = 128
			}
			out.WriteByte(code)
			out.WriteByte(byte(chunk - 1))
			n -= chunk
		}
	}
	writeRun(colorCode, runLen)
	writeRun(0x62, total-runLen)
	return out.Bytes()
}

func TestConvertNoteToPNGs_FooterPagesAndLayerVisibility(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer

	// Write Supernote header where FINALOPERATION_PAGE is 1, but footer actually has 3 pages
	buf.WriteString("noteSN_FILE_VER_20230015")
	appendBlock(&buf, []byte("<MODULE_LABEL:none><FILE_TYPE:NOTE><FINALOPERATION_PAGE:1>"))

	// Bitmap 1: Black (0x61) for first 100 pixels
	bmpBlackOff := appendBlock(&buf, buildRattaRLELayer(0x61, 100))
	// Bitmap 2: Dark gray (0x63) for first 200 pixels
	bmpGrayOff := appendBlock(&buf, buildRattaRLELayer(0x63, 200))
	// Bitmap 3: Transparent background (0x62)
	bmpBgOff := appendBlock(&buf, buildRattaRLELayer(0x62, 0))

	bgLayerOff := appendBlock(&buf, []byte(fmt.Sprintf("<LAYERTYPE:NOTE><LAYERPROTOCOL:RATTA_RLE><LAYERNAME:BGLAYER><LAYERBITMAP:%d>", bmpBgOff)))
	mainLayerBlackOff := appendBlock(&buf, []byte(fmt.Sprintf("<LAYERTYPE:NOTE><LAYERPROTOCOL:RATTA_RLE><LAYERNAME:MAINLAYER><LAYERBITMAP:%d>", bmpBlackOff)))
	layer1GrayOff := appendBlock(&buf, []byte(fmt.Sprintf("<LAYERTYPE:NOTE><LAYERPROTOCOL:RATTA_RLE><LAYERNAME:LAYER1><LAYERBITMAP:%d>", bmpGrayOff)))
	layer2HiddenOff := appendBlock(&buf, []byte(fmt.Sprintf("<LAYERTYPE:NOTE><LAYERPROTOCOL:RATTA_RLE><LAYERNAME:LAYER2><LAYERBITMAP:%d>", bmpBlackOff)))

	// Page 1: LAYER1 (visible, gray 200px) on top of MAINLAYER (visible, black 100px), LAYER2 is hidden (isVisible#false)
	page1Meta := fmt.Sprintf(
		`<LAYERINFO:[{"layerId"#2,"isVisible"#false,"isDeleted"#false},{"layerId"#1,"isVisible"#true,"isDeleted"#false},{"layerId"#0,"isVisible"#true,"isDeleted"#false},{"layerId"#-1,"isVisible"#true,"isDeleted"#false}]><LAYERSEQ:LAYER2,LAYER1,MAINLAYER,BGLAYER><BGLAYER:%d><MAINLAYER:%d><LAYER1:%d><LAYER2:%d><ORIENTATION:1000>`,
		bgLayerOff, mainLayerBlackOff, layer1GrayOff, layer2HiddenOff,
	)
	page1Off := appendBlock(&buf, []byte(page1Meta))

	// Page 2: MAINLAYER (black 100px) on top of LAYER1 (gray 200px)
	page2Meta := fmt.Sprintf(
		`<LAYERSEQ:MAINLAYER,LAYER1,BGLAYER><BGLAYER:%d><MAINLAYER:%d><LAYER1:%d><ORIENTATION:1000>`,
		bgLayerOff, mainLayerBlackOff, layer1GrayOff,
	)
	page2Off := appendBlock(&buf, []byte(page2Meta))

	// Page 3: Landscape orientation (1090) -> 1872x1404
	page3Meta := fmt.Sprintf(
		`<LAYERSEQ:MAINLAYER,BGLAYER><BGLAYER:%d><MAINLAYER:%d><ORIENTATION:1090>`,
		bgLayerOff, mainLayerBlackOff,
	)
	page3Off := appendBlock(&buf, []byte(page3Meta))

	footerMeta := fmt.Sprintf("<PAGE1:%d><PAGE2:%d><PAGE3:%d>", page1Off, page2Off, page3Off)
	footerOff := appendBlock(&buf, []byte(footerMeta))

	var footerPtr [4]byte
	binary.LittleEndian.PutUint32(footerPtr[:], footerOff)
	buf.Write(footerPtr[:])

	pngs, err := sync.ConvertNoteToPNGs(ctx, "SyntheticMultiPage", buf.Bytes())
	if err != nil {
		t.Fatalf("ConvertNoteToPNGs failed: %v", err)
	}
	if len(pngs) != 3 {
		t.Fatalf("expected 3 pages from footer table (ignoring FINALOPERATION_PAGE:1), got %d", len(pngs))
	}

	// Page 1: LAYER1 (gray 0x9d) painted over MAINLAYER (black 0x00) for first 200 pixels
	img1, err := png.Decode(bytes.NewReader(pngs[0]))
	if err != nil {
		t.Fatalf("failed to decode page 1: %v", err)
	}
	r1, _, _, _ := img1.At(10, 0).RGBA()
	if r1>>8 != 0x9d {
		t.Errorf("page 1 pixel (10,0): expected LAYER1 dark gray 0x9d, got 0x%02x", r1>>8)
	}

	// Page 2: MAINLAYER (black 0x00 for 0..99) painted over LAYER1 (gray 0x9d for 0..199)
	img2, err := png.Decode(bytes.NewReader(pngs[1]))
	if err != nil {
		t.Fatalf("failed to decode page 2: %v", err)
	}
	r2a, _, _, _ := img2.At(10, 0).RGBA()
	if r2a>>8 != 0x00 {
		t.Errorf("page 2 pixel (10,0): expected MAINLAYER black 0x00, got 0x%02x", r2a>>8)
	}
	r2b, _, _, _ := img2.At(150, 0).RGBA()
	if r2b>>8 != 0x9d {
		t.Errorf("page 2 pixel (150,0): expected underlying LAYER1 dark gray 0x9d, got 0x%02x", r2b>>8)
	}

	// Page 3: Landscape orientation 1872x1404
	img3, err := png.Decode(bytes.NewReader(pngs[2]))
	if err != nil {
		t.Fatalf("failed to decode page 3: %v", err)
	}
	if img3.Bounds().Dx() != 1872 || img3.Bounds().Dy() != 1404 {
		t.Errorf("page 3: expected landscape dimensions 1872x1404, got %dx%d", img3.Bounds().Dx(), img3.Bounds().Dy())
	}
}
