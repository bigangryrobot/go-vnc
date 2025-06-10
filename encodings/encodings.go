/*
Package encodings provides constants for the known VNC encoding types.
https://tools.ietf.org/html/rfc6143#section-7.7
*/
package encodings

// Encoding represents a known VNC encoding type.
type Encoding int32

//go:generate stringer -type=Encoding

const (
	// Standard Encodings
	Raw      Encoding = 0
	CopyRect Encoding = 1
	RRE      Encoding = 2
	CoRRE    Encoding = 4
	Hextile  Encoding = 5
	Zlib     Encoding = 6
	Tight    Encoding = 7
	ZlibHex  Encoding = 8
	TRLE     Encoding = 15
	ZRLE     Encoding = 16
	Hitachi  Encoding = 17

	// Pseudo Encodings (negative numbers)
	CursorPseudo              Encoding = -239
	DesktopSizePseudo         Encoding = -223
	ExtendedDesktopSizePseudo Encoding = -308
	DesktopNamePseudo         Encoding = -307
	FencePseudo               Encoding = -312
	ContinuousUpdatesPseudo   Encoding = -313
)
