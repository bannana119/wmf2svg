package enum

const (
	META_SETBKMODE           = 0x0102
	META_SETMAPMODE          = 0x0103
	META_SETROP2             = 0x0104
	META_SETPOLYFILLMODE     = 0x0106
	META_SETWINDOWORG        = 0x020B
	META_SETWINDOWEXT        = 0x020C
	META_SETVIEWPORTORG      = 0x020D
	META_SETVIEWPORTEXT      = 0x020E
	META_OFFSETWINDOWORG     = 0x020F
	META_SCALEWINDOWEXT      = 0x0410
	META_OFFSETVIEWPORTORG   = 0x0211
	META_SCALEVIEWPORTEXT    = 0x0412
	META_LINETO              = 0x0213
	META_MOVETO              = 0x0214
	META_EXCLUDECLIPRECT     = 0x0415
	META_INTERSECTCLIPRECT   = 0x0416
	META_OFFSETVIEWPORTORG2  = 0x0417
	META_OFFSETCLIPRECT      = 0x0218
	META_FRAMEREGION         = 0x0429
	META_ANIMATEPALETTE      = 0x0436
	META_TEXTOUT             = 0x0521
	META_POLYGON             = 0x0324
	META_POLYLINE            = 0x0325
	META_BITBLT              = 0x0922
	META_STRETCHBLT          = 0x0F43
	META_PIE                 = 0x081A
	META_ESCAPE              = 0x0626
	META_RECTANGLE           = 0x041B
	META_PATBLT              = 0x061D
	META_ELLIPSE             = 0x0418
	META_FLOODFILL           = 0x0419
	META_ROUNDRECT           = 0x061C
	META_ARC                 = 0x0817
	META_CHORD               = 0x0830
	META_REALIZEPALETTE      = 0x0035
	META_SELECTOBJECT        = 0x012D
	META_CREATEPENINDIRECT   = 0x02FA
	META_CREATEBRUSHINDIRECT = 0x02FC
	META_CREATEFONTINDIRECT  = 0x02FB
	META_DELETEOBJECT        = 0x01F0
	META_DIBBITBLT           = 0x0940
	META_DIBSTRETCHBLT       = 0x0F43
	META_EXTTEXTOUT          = 0x0A32
	META_SETTEXTALIGN        = 0x022E
	META_SETTEXTCOLOR        = 0x0209
	META_SETPENWIDTH         = 0x0311
)

type Point struct {
	X, Y int32
}

type PenColor struct{ R, G, B uint8 }

type WmfState struct {
	PenColor       PenColor
	BrushColor     PenColor
	TextColor      PenColor
	BkMode         int32
	PenWidth       int32
	PenHeight      int32
	PolyFillMode   int32
	MapMode        int32
	WinOrg         Point
	WinExt         Point
	ViewportOrg    Point
	ViewportExt    Point
	CurrentPos     Point
	CurrentFont    string
	CurrentFontSz  float64
	CurrentItalic  bool
	CurrentCharset byte
	Objects        map[int32]GdiObject
}

type GdiObject struct {
	Type     string
	FontName string
	FontSize float64
	Italic   bool
	Charset  byte
}

type SvgElement struct {
	Tag       string
	Attrs     map[string]string
	AttrOrder []string
	Inner     string
	ZOrder    int
}
