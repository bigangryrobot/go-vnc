/*
Implementation of RFC 6143 §7.7 & §7.8 Encodings.
https://tools.ietf.org/html/rfc6143#section-7.7
*/
package vnc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/bigangryrobot/go-vnc/encodings"
)

//=============================================================================
// Encodings

// An Encoding implements a method for encoding pixel data that is
// sent by the server to the client.
type Encoding interface {
	fmt.Stringer
	Marshaler

	// Read the contents of the encoded pixel data from the reader.
	// This should return a new Encoding implementation that contains
	// the proper data.
	Read(*ClientConn, *Rectangle) (Encoding, error)

	// The number that uniquely identifies this encoding type.
	Type() encodings.Encoding
}

// Encodings describes a slice of Encoding.
type Encodings []Encoding

// Verify that interfaces are honored.
var _ Marshaler = (*Encodings)(nil)

// Marshal implements the Marshaler interface.
func (e Encodings) Marshal() ([]byte, error) {
	buf := NewBuffer(nil)
	for _, enc := range e {
		if err := buf.Write(enc.Type()); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

//-----------------------------------------------------------------------------
// Raw Encoding
//
// Raw encoding is the simplest encoding type, which is raw pixel data.
//
// See RFC 6143 §7.7.1.
// https://tools.ietf.org/html/rfc6143#section-7.7.1

// RawEncoding holds raw encoded rectangle data.
type RawEncoding struct {
	Colors []Color
}

// Verify that interfaces are honored.
var _ Encoding = (*RawEncoding)(nil)

// Marshal implements the Encoding interface.
func (e *RawEncoding) Marshal() ([]byte, error) {
	buf := NewBuffer(nil)

	for _, c := range e.Colors {
		bytes, err := c.Marshal()
		if err != nil {
			return nil, err
		}
		if err := buf.Write(bytes); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// Read implements the Encoding interface.
func (*RawEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	var buf bytes.Buffer
	bytesPerPixel := int(c.pixelFormat.BPP / 8)
	n := rect.Area() * bytesPerPixel
	if err := c.receiveN(&buf, n); err != nil {
		return nil, fmt.Errorf("unable to read rectangle with raw encoding: %w", err)
	}

	colors := make([]Color, rect.Area())
	for y := uint16(0); y < rect.Height; y++ {
		for x := uint16(0); x < rect.Width; x++ {
			color := NewColor(&c.pixelFormat, &c.colorMap)
			if err := color.Unmarshal(buf.Next(bytesPerPixel)); err != nil {
				return nil, err
			}
			colors[int(y)*int(rect.Width)+int(x)] = *color
		}
	}

	return &RawEncoding{colors}, nil
}

// String implements the fmt.Stringer interface.
func (*RawEncoding) String() string { return "RawEncoding" }

// Type implements the Encoding interface.
func (*RawEncoding) Type() encodings.Encoding { return encodings.Raw }

// -----------------------------------------------------------------------------
// CopyRect Encoding
//
// CopyRect encoding is used to indicate that a rectangle of pixel data should
// be copied from another part of the framebuffer.
//
// See RFC 6143 §7.7.2.
// https://tools.ietf.org/html/rfc6143#section-7.7.2
type CopyRectEncoding struct {
	SrcX, SrcY uint16
}

// Verify that interfaces are honored.
var _ Encoding = (*CopyRectEncoding)(nil)

// Read implements the Encoding interface.
func (*CopyRectEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	var msg struct {
		SrcX uint16
		SrcY uint16
	}
	if err := binary.Read(c.Conn, binary.BigEndian, &msg); err != nil {
		return nil, fmt.Errorf("failed to read copyrect encoding: %w", err)
	}
	return &CopyRectEncoding{SrcX: msg.SrcX, SrcY: msg.SrcY}, nil
}

// String implements the fmt.Stringer interface.
func (e *CopyRectEncoding) String() string {
	return fmt.Sprintf("CopyRectEncoding(SrcX:%d, SrcY:%d)", e.SrcX, e.SrcY)
}

// Type implements the Encoding interface.
func (*CopyRectEncoding) Type() encodings.Encoding {
	return encodings.CopyRect
}

// Marshal implements the Marshaler interface.
func (e *CopyRectEncoding) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, e.SrcX); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, e.SrcY); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// -----------------------------------------------------------------------------
// RRE Encoding
//
// Rise and Run-length Encoding is for updates with large solid color blocks.
//
// See RFC 6143 §7.7.3.
// https://tools.ietf.org/html/rfc6143#section-7.7.3
type RRESubRect struct {
	Color Color
	Rect  Rectangle
}

type RREEncoding struct {
	BackgroundColor Color
	SubRects        []RRESubRect
}

// Verify that interfaces are honored.
var _ Encoding = (*RREEncoding)(nil)

// Read implements the Encoding interface.
func (*RREEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	var numberOfSubRects uint32
	if err := binary.Read(c.Conn, binary.BigEndian, &numberOfSubRects); err != nil {
		return nil, fmt.Errorf("RRE: failed to read sub-rectangle count: %w", err)
	}

	bytesPerPixel := int(c.pixelFormat.BPP / 8)

	// Read background color
	bgPixelBytes := make([]byte, bytesPerPixel)
	if _, err := io.ReadFull(c.Conn, bgPixelBytes); err != nil {
		return nil, fmt.Errorf("RRE: failed to read background color: %w", err)
	}
	bgColor := NewColor(&c.pixelFormat, &c.colorMap)
	if err := bgColor.Unmarshal(bgPixelBytes); err != nil {
		return nil, fmt.Errorf("RRE: failed to unmarshal background color: %w", err)
	}

	// Read sub-rectangles
	subRects := make([]RRESubRect, numberOfSubRects)
	for i := uint32(0); i < numberOfSubRects; i++ {
		subRectPixelBytes := make([]byte, bytesPerPixel)
		if _, err := io.ReadFull(c.Conn, subRectPixelBytes); err != nil {
			return nil, fmt.Errorf("RRE: failed to read sub-rect color %d: %w", i, err)
		}
		subRectColor := NewColor(&c.pixelFormat, &c.colorMap)
		if err := subRectColor.Unmarshal(subRectPixelBytes); err != nil {
			return nil, fmt.Errorf("RRE: failed to unmarshal sub-rect color %d: %w", i, err)
		}

		var subRectGeom struct {
			X, Y, W, H uint16
		}
		if err := binary.Read(c.Conn, binary.BigEndian, &subRectGeom); err != nil {
			return nil, fmt.Errorf("RRE: failed to read sub-rect geometry %d: %w", i, err)
		}

		subRects[i] = RRESubRect{
			Color: *subRectColor,
			Rect: Rectangle{
				X:      subRectGeom.X,
				Y:      subRectGeom.Y,
				Width:  subRectGeom.W,
				Height: subRectGeom.H,
			},
		}
	}

	return &RREEncoding{BackgroundColor: *bgColor, SubRects: subRects}, nil
}

// String implements the fmt.Stringer interface.
func (e *RREEncoding) String() string {
	return fmt.Sprintf("RREEncoding(%d sub-rects)", len(e.SubRects))
}

// Type implements the Encoding interface.
func (*RREEncoding) Type() encodings.Encoding {
	return encodings.RRE
}

// Marshal implements the Marshaler interface.
func (e *RREEncoding) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(e.SubRects))); err != nil {
		return nil, err
	}
	bgBytes, err := e.BackgroundColor.Marshal()
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(bgBytes); err != nil {
		return nil, err
	}

	for _, sr := range e.SubRects {
		srColorBytes, err := sr.Color.Marshal()
		if err != nil {
			return nil, err
		}
		if _, err := buf.Write(srColorBytes); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, &sr.Rect); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// -----------------------------------------------------------------------------
// Hextile Encoding
//
// See RFC 6143 §7.7.4
type HextileEncoding struct {
	Colors []Color
}

// Verify that interfaces are honored.
var _ Encoding = (*HextileEncoding)(nil)

func (*HextileEncoding) Type() encodings.Encoding { return encodings.Hextile }
func (e *HextileEncoding) String() string {
	return fmt.Sprintf("HextileEncoding(%d colors)", len(e.Colors))
}
func (*HextileEncoding) Marshal() ([]byte, error) {
	return nil, errors.New("client-side marshalling of HextileEncoding not supported: this is a server-to-client encoding")
}

// Read implements the Encoding interface for Hextile.
func (*HextileEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	colors := make([]Color, rect.Area())
	bytesPerPixel := int(c.pixelFormat.BPP / 8)
	var backgroundColor, foregroundColor Color

	for y := rect.Y; y < rect.Y+rect.Height; y += 16 {
		for x := rect.X; x < rect.X+rect.Width; x += 16 {
			tileW := uint16(16)
			tileH := uint16(16)

			if rect.X+rect.Width-x < 16 {
				tileW = rect.X + rect.Width - x
			}
			if rect.Y+rect.Height-y < 16 {
				tileH = rect.Y + rect.Height - y
			}

			var subencodingMask byte
			if err := binary.Read(c.Conn, binary.BigEndian, &subencodingMask); err != nil {
				return nil, fmt.Errorf("hextile: error reading subencoding mask: %w", err)
			}

			isRaw := (subencodingMask & 0x01) != 0
			if isRaw {
				rawTileData := make([]byte, int(tileW)*int(tileH)*bytesPerPixel)
				if _, err := io.ReadFull(c.Conn, rawTileData); err != nil {
					return nil, fmt.Errorf("hextile: failed to read raw tile: %w", err)
				}
				buf := bytes.NewBuffer(rawTileData)
				for ty := uint16(0); ty < tileH; ty++ {
					for tx := uint16(0); tx < tileW; tx++ {
						color := NewColor(&c.pixelFormat, &c.colorMap)
						if err := color.Unmarshal(buf.Next(bytesPerPixel)); err != nil {
							return nil, fmt.Errorf("hextile: failed to unmarshal raw tile color: %w", err)
						}
						px := (x - rect.X) + tx
						py := (y - rect.Y) + ty
						index := int(py)*int(rect.Width) + int(px)
						if index < len(colors) {
							colors[index] = *color
						}
					}
				}
				continue
			}

			backgroundSpecified := (subencodingMask & 0x02) != 0
			if backgroundSpecified {
				bgBytes := make([]byte, bytesPerPixel)
				if _, err := io.ReadFull(c.Conn, bgBytes); err != nil {
					return nil, fmt.Errorf("hextile: failed to read background color: %w", err)
				}
				bgColor := NewColor(&c.pixelFormat, &c.colorMap)
				if err := bgColor.Unmarshal(bgBytes); err != nil {
					return nil, fmt.Errorf("hextile: failed to unmarshal background color: %w", err)
				}
				backgroundColor = *bgColor
			}

			foregroundSpecified := (subencodingMask & 0x04) != 0
			if foregroundSpecified {
				fgBytes := make([]byte, bytesPerPixel)
				if _, err := io.ReadFull(c.Conn, fgBytes); err != nil {
					return nil, fmt.Errorf("hextile: failed to read foreground color: %w", err)
				}
				fgColor := NewColor(&c.pixelFormat, &c.colorMap)
				if err := fgColor.Unmarshal(fgBytes); err != nil {
					return nil, fmt.Errorf("hextile: failed to unmarshal foreground color: %w", err)
				}
				foregroundColor = *fgColor
			}

			// Fill tile with background color
			for ty := uint16(0); ty < tileH; ty++ {
				for tx := uint16(0); tx < tileW; tx++ {
					px := (x - rect.X) + tx
					py := (y - rect.Y) + ty
					index := int(py)*int(rect.Width) + int(px)
					if index < len(colors) {
						colors[index] = backgroundColor
					}
				}
			}

			anySubrects := (subencodingMask & 0x08) != 0
			if anySubrects {
				var numberOfSubRects byte
				if err := binary.Read(c.Conn, binary.BigEndian, &numberOfSubRects); err != nil {
					return nil, fmt.Errorf("hextile: failed to read sub-rectangle count: %w", err)
				}
				subrectsColoured := (subencodingMask & 0x10) != 0

				for i := 0; i < int(numberOfSubRects); i++ {
					var subRectColor Color
					if subrectsColoured {
						srColorBytes := make([]byte, bytesPerPixel)
						if _, err := io.ReadFull(c.Conn, srColorBytes); err != nil {
							return nil, fmt.Errorf("hextile: failed to read subrect color: %w", err)
						}
						srColor := NewColor(&c.pixelFormat, &c.colorMap)
						if err := srColor.Unmarshal(srColorBytes); err != nil {
							return nil, fmt.Errorf("hextile: failed to unmarshal subrect color: %w", err)
						}
						subRectColor = *srColor
					} else {
						subRectColor = foregroundColor
					}

					var xy, wh byte
					if err := binary.Read(c.Conn, binary.BigEndian, &xy); err != nil {
						return nil, fmt.Errorf("hextile: failed to read subrect geometry xy: %w", err)
					}
					if err := binary.Read(c.Conn, binary.BigEndian, &wh); err != nil {
						return nil, fmt.Errorf("hextile: failed to read subrect geometry wh: %w", err)
					}

					subX := (xy >> 4) & 0x0F
					subY := xy & 0x0F
					subW := ((wh >> 4) & 0x0F) + 1
					subH := (wh & 0x0F) + 1

					for sy := uint16(0); sy < uint16(subH); sy++ {
						for sx := uint16(0); sx < uint16(subW); sx++ {
							px := (x - rect.X) + uint16(subX) + sx
							py := (y - rect.Y) + uint16(subY) + sy
							index := int(py)*int(rect.Width) + int(px)
							if index < len(colors) {
								colors[index] = subRectColor
							}
						}
					}
				}
			}
		}
	}
	return &HextileEncoding{Colors: colors}, nil
}

// -----------------------------------------------------------------------------
// ZRLE Encoding
//
// Zlib Run-Length Encoding is an efficient compressed encoding.
//
// See RFC 6143 §7.7.6.
// https://tools.ietf.org/html/rfc6143#section-7.7.6
type ZRLEEncoding struct {
	// Data holds the decompressed ZRLE data.
	Data []byte
}

// Verify that interfaces are honored.
var _ Encoding = (*ZRLEEncoding)(nil)

// Read implements the Encoding interface.
func (*ZRLEEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	var dataLen uint32
	if err := binary.Read(c.Conn, binary.BigEndian, &dataLen); err != nil {
		return nil, fmt.Errorf("ZRLE: failed to read data length: %w", err)
	}

	if dataLen == 0 {
		return &ZRLEEncoding{Data: []byte{}}, nil
	}

	compressedDataReader := io.LimitReader(c.Conn, int64(dataLen))
	zlibReader, err := zlib.NewReader(compressedDataReader)
	if err != nil {
		return nil, fmt.Errorf("ZRLE: failed to create zlib reader: %w", err)
	}
	defer zlibReader.Close()

	decompressedData, err := io.ReadAll(zlibReader)
	if err != nil {
		return nil, fmt.Errorf("ZRLE: failed to decompress data: %w", err)
	}

	return &ZRLEEncoding{Data: decompressedData}, nil
}

// String implements the fmt.Stringer interface.
func (e *ZRLEEncoding) String() string {
	return fmt.Sprintf("ZRLEEncoding(%d bytes decompressed)", len(e.Data))
}

// Type implements the Encoding interface.
func (*ZRLEEncoding) Type() encodings.Encoding {
	return encodings.ZRLE
}

// Marshal implements the Marshaler interface.
func (e *ZRLEEncoding) Marshal() ([]byte, error) {
	var compressedData bytes.Buffer
	w := zlib.NewWriter(&compressedData)
	if _, err := w.Write(e.Data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint32(compressedData.Len())); err != nil {
		return nil, err
	}
	if _, err := buf.Write(compressedData.Bytes()); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// -----------------------------------------------------------------------------
// Tight Encoding
//
// See RFC 6143 §7.7.7
type TightEncoding struct {
	Data []byte
}

// Verify that interfaces are honored.
var _ Encoding = (*TightEncoding)(nil)

func (*TightEncoding) Type() encodings.Encoding { return encodings.Tight }
func (*TightEncoding) String() string           { return "TightEncoding" }
func (*TightEncoding) Marshal() ([]byte, error) {
	return nil, errors.New("client-side marshalling of TightEncoding not supported: this is a server-to-client encoding")
}

// Read implements the Encoding interface for Tight encoding.
func (e *TightEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	var subencoding byte
	if err := binary.Read(c.Conn, binary.BigEndian, &subencoding); err != nil {
		return nil, fmt.Errorf("tight: failed to read subencoding: %w", err)
	}

	// Reset zlib streams if necessary
	for i := 0; i < 4; i++ {
		if (subencoding>>uint(i))&1 != 0 {
			if c.zlibs[i] != nil {
				c.zlibs[i].Close()
				c.zlibs[i] = nil
			}
		}
	}

	filterID := (subencoding >> 4) & 0x0F

	if filterID == 8 { // JPEG
		return nil, errors.New("tight JPEG encoding not supported")
	}

	if filterID > 2 {
		return nil, fmt.Errorf("tight: unsupported filter ID: %d", filterID)
	}

	return e.readTightFilter(c, rect, filterID)
}

func (e *TightEncoding) readTightFilter(c *ClientConn, rect *Rectangle, filterID byte) (Encoding, error) {
	switch filterID {
	case 0: // Copy filter
		return e.readTightCopy(c, rect)
	case 1: // Palette filter
		return e.readTightPalette(c, rect)
	case 2: // Gradient filter
		return e.readTightGradient(c, rect)
	}
	return nil, fmt.Errorf("tight: unexpected filter ID: %d", filterID)
}

func (e *TightEncoding) readTightCopy(c *ClientConn, rect *Rectangle) (Encoding, error) {
	bytesPerPixel := (c.pixelFormat.BPP + 7) / 8
	uncompressedSize := int(rect.Width) * int(rect.Height) * int(bytesPerPixel)

	data, err := e.readCompressedData(c, 0)
	if err != nil {
		return nil, fmt.Errorf("tight (copy): %w", err)
	}

	if len(data) != uncompressedSize {
		return nil, fmt.Errorf("tight (copy): decompressed data size mismatch (got %d, want %d)", len(data), uncompressedSize)
	}

	return &TightEncoding{Data: data}, nil
}

func (e *TightEncoding) readTightPalette(c *ClientConn, rect *Rectangle) (Encoding, error) {
	var paletteSizeMinus1 byte
	if err := binary.Read(c.Conn, binary.BigEndian, &paletteSizeMinus1); err != nil {
		return nil, fmt.Errorf("tight (palette): failed to read palette size: %w", err)
	}
	paletteSize := int(paletteSizeMinus1) + 1
	bytesPerPixel := int(c.pixelFormat.BPP / 8)

	palette := make([]Color, paletteSize)
	for i := 0; i < paletteSize; i++ {
		colorBytes := make([]byte, bytesPerPixel)
		if _, err := io.ReadFull(c.Conn, colorBytes); err != nil {
			return nil, fmt.Errorf("tight (palette): failed to read color %d: %w", i, err)
		}
		color := NewColor(&c.pixelFormat, &c.colorMap)
		if err := color.Unmarshal(colorBytes); err != nil {
			return nil, err
		}
		palette[i] = *color
	}

	data, err := e.readCompressedData(c, 1)
	if err != nil {
		return nil, fmt.Errorf("tight (palette): %w", err)
	}

	pixelData := new(bytes.Buffer)
	expectedSize := int(rect.Width) * int(rect.Height) * bytesPerPixel
	pixelData.Grow(expectedSize)
	pixelsWritten := 0
	totalPixels := int(rect.Width) * int(rect.Height)

	if paletteSize <= 2 {
		// Monochrome bitmap processing
		for _, byteVal := range data {
			for i := 7; i >= 0; i-- {
				if pixelsWritten >= totalPixels {
					break
				}
				index := (byteVal >> uint(i)) & 1
				colorBytes, err := palette[index].Marshal()
				if err != nil {
					return nil, fmt.Errorf("tight (palette): failed to marshal color from palette: %w", err)
				}
				pixelData.Write(colorBytes)
				pixelsWritten++
			}
			if pixelsWritten >= totalPixels {
				break
			}
		}
	} else {
		// Indexed color processing
		for _, index := range data {
			if int(index) >= len(palette) {
				return nil, fmt.Errorf("tight (palette): invalid palette index %d for palette of size %d", index, len(palette))
			}
			colorBytes, err := palette[index].Marshal()
			if err != nil {
				return nil, fmt.Errorf("tight (palette): failed to marshal color from palette: %w", err)
			}
			pixelData.Write(colorBytes)
		}
	}

	if pixelData.Len() != expectedSize {
		return nil, fmt.Errorf("tight (palette): expanded data size mismatch (got %d, want %d)", pixelData.Len(), expectedSize)
	}

	return &TightEncoding{Data: pixelData.Bytes()}, nil
}

func (e *TightEncoding) readTightGradient(c *ClientConn, rect *Rectangle) (Encoding, error) {
	bytesPerPixel := int(c.pixelFormat.BPP / 8)
	if bytesPerPixel != 3 && bytesPerPixel != 4 {
		return nil, fmt.Errorf("tight (gradient): unsupported bytesPerPixel: %d", bytesPerPixel)
	}

	correctionData, err := e.readCompressedData(c, 2)
	if err != nil {
		return nil, fmt.Errorf("tight (gradient): %w", err)
	}
	correctionReader := bytes.NewReader(correctionData)

	pixelData := make([]byte, int(rect.Width)*int(rect.Height)*bytesPerPixel)

	for y := uint16(0); y < rect.Height; y++ {
		for x := uint16(0); x < rect.Width; x++ {
			var p1, p2, p3 [4]byte // R, G, B, (A)

			// Get pixel to the left (P1)
			if x > 0 {
				offset := ((y * rect.Width) + (x - 1)) * uint16(bytesPerPixel)
				copy(p1[:], pixelData[offset:offset+uint16(bytesPerPixel)])
			}
			// Get pixel above (P2)
			if y > 0 {
				offset := (((y - 1) * rect.Width) + x) * uint16(bytesPerPixel)
				copy(p2[:], pixelData[offset:offset+uint16(bytesPerPixel)])
			}
			// Get pixel top-left (P3)
			if x > 0 && y > 0 {
				offset := (((y - 1) * rect.Width) + (x - 1)) * uint16(bytesPerPixel)
				copy(p3[:], pixelData[offset:offset+uint16(bytesPerPixel)])
			}

			currentPixelOffset := ((y * rect.Width) + x) * uint16(bytesPerPixel)

			for b := 0; b < bytesPerPixel; b++ {
				pred := int(p1[b]) + int(p2[b]) - int(p3[b])

				// Clamp predictor
				if pred < 0 {
					pred = 0
				}
				if pred > 255 {
					pred = 255
				}

				correction, err := correctionReader.ReadByte()
				if err != nil {
					return nil, fmt.Errorf("tight (gradient): failed to read correction byte: %w", err)
				}

				pixelData[currentPixelOffset+uint16(b)] = byte(pred) + correction
			}
		}
	}

	return &TightEncoding{Data: pixelData}, nil
}

// readCompressedData reads a compact length, then that many bytes of zlib data.
func (e *TightEncoding) readCompressedData(c *ClientConn, zlibStream int) ([]byte, error) {
	// Read compact length
	var length int
	for i := 0; i < 3; i++ {
		var part byte
		if err := binary.Read(c.Conn, binary.BigEndian, &part); err != nil {
			return nil, fmt.Errorf("failed to read compact length part %d: %w", i, err)
		}
		length |= int(part&0x7F) << (i * 7)
		if (part & 0x80) == 0 {
			break
		}
	}

	if length == 0 {
		return []byte{}, nil
	}

	compressedData := make([]byte, length)
	if _, err := io.ReadFull(c.Conn, compressedData); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Initialize zlib reader if it's the first time
	if c.zlibs[zlibStream] == nil {
		r, err := zlib.NewReader(bytes.NewReader(compressedData))
		if err != nil {
			return nil, fmt.Errorf("failed to create new zlib reader: %w", err)
		}
		c.zlibs[zlibStream] = r
	} else {
		// Reset the reader with the new data
		err := c.zlibs[zlibStream].(zlib.Resetter).Reset(bytes.NewReader(compressedData), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to reset zlib reader: %w", err)
		}
	}

	decompressed, err := io.ReadAll(c.zlibs[zlibStream])
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return decompressed, nil
}

//=============================================================================
// Pseudo-Encodings
//
// Rectangles with a "pseudo-encoding" allow a server to send data to the
// client. The interpretation of the data depends on the pseudo-encoding.
//
// See RFC 6143 §7.8.
// https://tools.ietf.org/html/rfc6143#section-7.8

// -----------------------------------------------------------------------------
// Cursor Pseudo-Encoding
//
// Used to transmit the shape of the remote cursor. The rectangle of the update
// defines the hotspot of the cursor.
//
// See RFC 6143 §7.8.1.
// https://tools.ietf.org/html/rfc6143#section-7.8.1
type CursorPseudoEncoding struct {
	Pixels  []byte
	Bitmask []byte
}

// Verify that interfaces are honored.
var _ Encoding = (*CursorPseudoEncoding)(nil)

// Read implements the Encoding interface.
func (*CursorPseudoEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	bytesPerPixel := int(c.pixelFormat.BPP / 8)
	area := int(rect.Width) * int(rect.Height)
	pixelDataSize := area * bytesPerPixel
	bitmaskSize := (int(rect.Width) + 7) / 8 * int(rect.Height)

	pixels := make([]byte, pixelDataSize)
	if _, err := io.ReadFull(c.Conn, pixels); err != nil {
		return nil, fmt.Errorf("failed to read cursor pixel data: %w", err)
	}

	bitmask := make([]byte, bitmaskSize)
	if _, err := io.ReadFull(c.Conn, bitmask); err != nil {
		return nil, fmt.Errorf("failed to read cursor bitmask data: %w", err)
	}

	return &CursorPseudoEncoding{Pixels: pixels, Bitmask: bitmask}, nil
}

// String implements the fmt.Stringer interface.
func (e *CursorPseudoEncoding) String() string {
	return "CursorPseudoEncoding"
}

// Type implements the Encoding interface.
func (*CursorPseudoEncoding) Type() encodings.Encoding {
	return encodings.CursorPseudo
}

// Marshal implements the Marshaler interface.
func (e *CursorPseudoEncoding) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.Write(e.Pixels); err != nil {
		return nil, err
	}
	if _, err := buf.Write(e.Bitmask); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//-----------------------------------------------------------------------------
// DesktopSize Pseudo-Encoding
//
// When a client requests DesktopSize pseudo-encoding, it is indicating to the
// server that it can handle changes to the framebuffer size. If this encoding
// received, the client must resize its framebuffer.
//
// See RFC 6143 §7.8.2.
// https://tools.ietf.org/html/rfc6143#section-7.8.2

// DesktopSizePseudoEncoding represents a desktop size message from the server.
type DesktopSizePseudoEncoding struct{}

// Verify that interfaces are honored.
var _ Encoding = (*DesktopSizePseudoEncoding)(nil)

// Marshal implements the Marshaler interface.
func (*DesktopSizePseudoEncoding) Marshal() ([]byte, error) {
	return []byte{}, nil
}

// Read implements the Encoding interface.
func (*DesktopSizePseudoEncoding) Read(c *ClientConn, rect *Rectangle) (Encoding, error) {
	c.fbWidth = rect.Width
	c.fbHeight = rect.Height

	return &DesktopSizePseudoEncoding{}, nil
}

// String implements the fmt.Stringer interface.
func (*DesktopSizePseudoEncoding) String() string { return "DesktopSizePseudoEncoding" }

// Type implements the Encoding interface.
func (*DesktopSizePseudoEncoding) Type() encodings.Encoding { return encodings.DesktopSizePseudo }
