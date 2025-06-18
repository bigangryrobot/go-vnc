/*
Package encodings provides constants for the known VNC encoding types.
https://tools.ietf.org/html/rfc6143#section-7.7
*/
package encodings

// Encoding represents a known VNC encoding type.
type EncodingType int32

//go:generate stringer -type=EncodingType

const (
	// Standard Encodings
	EncRaw      EncodingType = 0
	EncCopyRect EncodingType = 1
	EncRRE      EncodingType = 2
	EncCoRRE    EncodingType = 4
	EncHextile  EncodingType = 5
	EncZlib     EncodingType = 6
	EncTight    EncodingType = 7
	EncZlibHex  EncodingType = 8
	EncTRLE     EncodingType = 15
	EncZRLE     EncodingType = 16
	EncHitachi  EncodingType = 17

	// Vendor-specific Encodings
	EncAtenAST2100             EncodingType = 0x57
	EncAtenASTJPEG             EncodingType = 0x58
	EncAtenHermon              EncodingType = 0x59
	EncAtenYarkon              EncodingType = 0x60
	EncAtenPilot3              EncodingType = 0x61
	EncUltra1                  EncodingType = 9
	EncUltra2                  EncodingType = 10
	EncJPEG                    EncodingType = 21
	EncJRLE                    EncodingType = 22
	EncTightPng                EncodingType = -260
	EncClientRedirect          EncodingType = -311
	EncExtendedClipboardPseudo EncodingType = -1063131698 //C0A1E5CE

	// Pseudo Encodings
	EncCursorPseudo              EncodingType = -239
	EncXCursorPseudo             EncodingType = -240
	EncPointerPosPseudo          EncodingType = -232
	EncDesktopSizePseudo         EncodingType = -223
	EncLastRectPseudo            EncodingType = -224
	EncExtendedDesktopSizePseudo EncodingType = -308
	EncDesktopNamePseudo         EncodingType = -307
	EncFencePseudo               EncodingType = -312
	EncContinuousUpdatesPseudo   EncodingType = -313
	EncXvpPseudo                 EncodingType = -309

	// QEMU specific
	EncQEMUPointerMotionChangePseudo EncodingType = -257
	EncQEMUExtendedKeyEventPseudo    EncodingType = -258

	// Compression Level Pseudo Encodings
	EncCompressionLevel1  EncodingType = -256
	EncCompressionLevel2  EncodingType = -255
	EncCompressionLevel3  EncodingType = -254
	EncCompressionLevel4  EncodingType = -253
	EncCompressionLevel5  EncodingType = -252
	EncCompressionLevel6  EncodingType = -251
	EncCompressionLevel7  EncodingType = -250
	EncCompressionLevel8  EncodingType = -249
	EncCompressionLevel9  EncodingType = -248
	EncCompressionLevel10 EncodingType = -247

	// JPEG Quality Level Pseudo Encodings
	EncJPEGQualityLevelPseudo1  EncodingType = -32
	EncJPEGQualityLevelPseudo2  EncodingType = -31
	EncJPEGQualityLevelPseudo3  EncodingType = -30
	EncJPEGQualityLevelPseudo4  EncodingType = -29
	EncJPEGQualityLevelPseudo5  EncodingType = -28
	EncJPEGQualityLevelPseudo6  EncodingType = -27
	EncJPEGQualityLevelPseudo7  EncodingType = -26
	EncJPEGQualityLevelPseudo8  EncodingType = -25
	EncJPEGQualityLevelPseudo9  EncodingType = -24
	EncJPEGQualityLevelPseudo10 EncodingType = -23

	// Register some common names for these as helpers
	RawEncoding                       = EncRaw
	CopyRectEncoding                  = EncCopyRect
	RREEncoding                       = EncRRE
	CoRREEncoding                     = EncCoRRE
	HextileEncoding                   = EncHextile
	ZlibEncoding                      = EncZlib
	TightEncoding                     = EncTight
	ZlibHexEncoding                   = EncZlibHex
	TRLEEncoding                      = EncTRLE
	ZRLEEncoding                      = EncZRLE
	HitachiEncoding                   = EncHitachi
	CursorPseudoEncoding              = EncCursorPseudo
	DesktopSizePseudoEncoding         = EncDesktopSizePseudo
	ExtendedDesktopSizePseudoEncoding = EncExtendedDesktopSizePseudo
	DesktopNamePseudoEncoding         = EncDesktopNamePseudo
	FencePseudoEncoding               = EncFencePseudo
	ContinuousUpdatesPseudoEncoding   = EncContinuousUpdatesPseudo
)
