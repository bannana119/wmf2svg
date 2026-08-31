package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"wmf2svg/enum"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func convertWmfToSvg(data []byte) (string, error) {
	if len(data) < 22 {
		return "", fmt.Errorf("文件太小，不是有效的WMF文件")
	}

	// 1. Parse Aldus Placeable Metafile Header
	// Magic is uint32 = 0x9AC6CDD7 at offset 0
	var aldusLeft, aldusTop, aldusRight, aldusBottom int32
	var hasAldusHeader bool
	recordStart := 0

	if len(data) >= 22 {
		magic32 := binary.LittleEndian.Uint32(data[0:4])
		if magic32 == 0x9AC6CDD7 {
			hasAldusHeader = true
			// Standard Aldus header field order:
			// magic(4) + hmf(2) + left(2) + top(2) + right(2) + bottom(2) + inch(2) + reserved(4) + checksum(2) = 22
			aldusLeft = int32(int16(binary.LittleEndian.Uint16(data[6:8])))
			aldusTop = int32(int16(binary.LittleEndian.Uint16(data[8:10])))
			aldusRight = int32(int16(binary.LittleEndian.Uint16(data[10:12])))
			aldusBottom = int32(int16(binary.LittleEndian.Uint16(data[12:14])))

			// METAHEADER starts at offset 22
			// mtType at 22, mtHeaderSize at 24
			mtHeaderSize := int32(binary.LittleEndian.Uint16(data[24:26]))
			// Records start after the METAHEADER (mtHeaderSize is in 16-bit words)
			recordStart = 22 + int(mtHeaderSize)*2
		}
	}

	// 2. Use Aldus header dimensions to compute output size
	// Fixed output width=768, height proportional
	aldusW := abs32(aldusRight - aldusLeft)
	aldusH := abs32(aldusBottom - aldusTop)
	if aldusW <= 0 {
		aldusW = 1000
	}
	if aldusH <= 0 {
		aldusH = 1000
	}

	// Output dimensions: fixed width 768, height proportional
	imgW := int32(768)
	imgH := int32(float64(aldusH) * 768.0 / float64(aldusW))
	if imgH <= 0 {
		imgH = 1
	}

	// Scale factors: map WMF coords to SVG pixels
	scaleX := float64(imgW) / float64(aldusW)
	scaleY := float64(imgH) / float64(aldusH)

	// Fallback if no Aldus header
	if !hasAldusHeader {
		recordStart = 0
		scaleX = 1
		scaleY = 1
		imgW = aldusW
		imgH = aldusH
	}

	state := enum.WmfState{
		PenColor:     enum.PenColor{0, 0, 0},
		BrushColor:   enum.PenColor{255, 255, 255},
		TextColor:    enum.PenColor{0, 0, 0},
		BkMode:       1,
		PenWidth:     1,
		PenHeight:    1,
		PolyFillMode: 1,
		MapMode:      1,
		Objects:      make(map[int32]enum.GdiObject),
	}
	var elements []enum.SvgElement
	zOrder := 0

	for offset := recordStart; offset+6 <= len(data); {
		rdSize := binary.LittleEndian.Uint32(data[offset : offset+4])
		if rdSize < 3 || int(rdSize)*2+offset > len(data) {
			break
		}
		rdFunction := binary.LittleEndian.Uint16(data[offset+4 : offset+6])
		recEnd := offset + int(rdSize)*2
		params := data[offset+6 : recEnd]

		switch rdFunction {
		case enum.META_MOVETO:
			if len(params) >= 4 {
				// WMF spec: PointS = y (int16), then x (int16)
				y := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				x := int32(int16(binary.LittleEndian.Uint16(params[2:4])))
				state.CurrentPos = enum.Point{x, y}
			}

		case enum.META_LINETO:
			if len(params) >= 4 {
				// WMF spec: PointS = y (int16), then x (int16)
				y := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				x := int32(int16(binary.LittleEndian.Uint16(params[2:4])))
				// Apply scale to map WMF coords to SVG pixels
				sx := float64(state.CurrentPos.X) * scaleX
				sy := float64(state.CurrentPos.Y) * scaleY
				ex := float64(x) * scaleX
				ey := float64(y) * scaleY
				sw := float64(state.PenWidth) * scaleY
				elements = append(elements, enum.SvgElement{
					Tag: "line",
					Attrs: map[string]string{
						"x1": fmt.Sprintf("%.6f", sx), "y1": fmt.Sprintf("%.6f", sy),
						"x2": fmt.Sprintf("%.6f", ex), "y2": fmt.Sprintf("%.6f", ey),
						"style": fmt.Sprintf("stroke-width:%.6f; stroke-linecap:round; stroke-linejoin:round; stroke-dasharray:none; stroke:black", sw),
					},
					AttrOrder: []string{"x1", "y1", "x2", "y2", "style"},
					ZOrder:    zOrder,
				})
				zOrder++
				state.CurrentPos = enum.Point{x, y}
			}

		case enum.META_POLYLINE:
			if len(params) >= 2 {
				numPts := int32(binary.LittleEndian.Uint16(params[0:2]))
				pts := parsePoints(params[2:], int(numPts))
				if len(pts) > 1 {
					var svgPts []string
					for _, p := range pts {
						svgPts = append(svgPts, fmt.Sprintf("%.6f,%.6f", float64(p.X)*scaleX, float64(p.Y)*scaleY))
					}
					elements = append(elements, enum.SvgElement{
						Tag: "polyline",
						Attrs: map[string]string{
							"points":       strings.Join(svgPts, " "),
							"stroke":       colorStr(state.PenColor),
							"stroke-width": fmt.Sprintf("%.6f", float64(max32(1, state.PenWidth))*scaleY),
							"fill":         "none",
						},
						ZOrder: zOrder,
					})
					zOrder++
				}
			}

		case enum.META_POLYGON:
			if len(params) >= 2 {
				numPts := int32(binary.LittleEndian.Uint16(params[0:2]))
				pts := parsePoints(params[2:], int(numPts))
				if len(pts) > 2 {
					var svgPts []string
					for _, p := range pts {
						svgPts = append(svgPts, fmt.Sprintf("%.6f,%.6f", float64(p.X)*scaleX, float64(p.Y)*scaleY))
					}
					fill := "none"
					if state.PolyFillMode != 0 {
						fill = colorStr(state.BrushColor)
					}
					elements = append(elements, enum.SvgElement{
						Tag: "polygon",
						Attrs: map[string]string{
							"points":       strings.Join(svgPts, " "),
							"stroke":       colorStr(state.PenColor),
							"stroke-width": fmt.Sprintf("%.6f", float64(max32(1, state.PenWidth))*scaleY),
							"fill":         fill,
						},
						ZOrder: zOrder,
					})
					zOrder++
				}
			}

		case enum.META_RECTANGLE:
			if len(params) >= 8 {
				left := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				top := int32(int16(binary.LittleEndian.Uint16(params[2:4])))
				right := int32(int16(binary.LittleEndian.Uint16(params[4:6])))
				bottom := int32(int16(binary.LittleEndian.Uint16(params[6:8])))
				w := math.Abs(float64(right-left) * scaleX)
				h := math.Abs(float64(bottom-top) * scaleY)
				x := math.Min(float64(left)*scaleX, float64(right)*scaleX)
				y := math.Min(float64(top)*scaleY, float64(bottom)*scaleY)
				elements = append(elements, enum.SvgElement{
					Tag: "rect",
					Attrs: map[string]string{
						"x": fmt.Sprintf("%.6f", x), "y": fmt.Sprintf("%.6f", y),
						"width": fmt.Sprintf("%.6f", w), "height": fmt.Sprintf("%.6f", h),
						"stroke": colorStr(state.PenColor), "fill": "none",
					},
					ZOrder: zOrder,
				})
				zOrder++
			}

		case enum.META_ELLIPSE:
			if len(params) >= 8 {
				left := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				top := int32(int16(binary.LittleEndian.Uint16(params[2:4])))
				right := int32(int16(binary.LittleEndian.Uint16(params[4:6])))
				bottom := int32(int16(binary.LittleEndian.Uint16(params[6:8])))
				cx := (float64(left) + float64(right)) / 2.0 * scaleX
				cy := (float64(top) + float64(bottom)) / 2.0 * scaleY
				rx := math.Abs(float64(right-left)) / 2.0 * scaleX
				ry := math.Abs(float64(bottom-top)) / 2.0 * scaleY
				elements = append(elements, enum.SvgElement{
					Tag: "ellipse",
					Attrs: map[string]string{
						"cx": fmt.Sprintf("%.6f", cx), "cy": fmt.Sprintf("%.6f", cy),
						"rx": fmt.Sprintf("%.6f", rx), "ry": fmt.Sprintf("%.6f", ry),
						"stroke": colorStr(state.PenColor), "fill": "none",
					},
					ZOrder: zOrder,
				})
				zOrder++
			}

		case enum.META_TEXTOUT, enum.META_EXTTEXTOUT:
			// Parse TEXTOUT/EXTTEXTOUT: params[0:2]=y, [2:4]=x, [4:6]=count, [6:8]=options(EXTTEXTOUT only), [8:8+count]=text, then Dx array
			var textX, textY int32
			strLen := 0
			headerSize := 6 // TEXTOUT: y(2)+x(2)+count(2)
			if rdFunction == enum.META_EXTTEXTOUT {
				headerSize = 8 // EXTTEXTOUT: y(2)+x(2)+count(2)+options(2)
			}
			if len(params) >= headerSize {
				textY = int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				textX = int32(int16(binary.LittleEndian.Uint16(params[2:4])))
				strLen = int(binary.LittleEndian.Uint16(params[4:6]))
			}
			if strLen <= 0 || len(params) < headerSize+strLen {
				break
			}

			textBytes := params[headerSize : headerSize+strLen]

			textDataEnd := headerSize + strLen
			if strLen%2 == 1 {
				textDataEnd++
			}
			var dxArray []int32
			if textDataEnd < len(params) {
				dxData := params[textDataEnd:]
				for i := 0; i+2 <= len(dxData); i += 2 {
					dxArray = append(dxArray, int32(int16(binary.LittleEndian.Uint16(dxData[i:i+2]))))
				}
			}

			// Use CurrentPos if textX/textY are 0
			if textX == 0 && textY == 0 {
				textX = state.CurrentPos.X
				textY = state.CurrentPos.Y
			}

			fontSize := state.CurrentFontSz
			if fontSize <= 0 {
				fontSize = 12
			}
			// Scale font size
			scaledFontSize := fontSize * scaleY
			fontFamily := state.CurrentFont
			if fontFamily == "" {
				fontFamily = "Times"
			}
			fontStyle := "normal"
			if state.CurrentItalic {
				fontStyle = "italic"
			}

			// If no Dx array or empty, output entire text as one element
			if len(dxArray) == 0 {
				charX := float64(textX) * scaleX
				charY := float64(textY) * scaleY
				var textContent string
				if state.CurrentCharset == 0x86 {
					// GBK 编码：整体解码为 UTF-8
					decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(textBytes)
					if err == nil {
						textContent = escapeXml(string(decoded))
					} else {
						textContent = escapeXml(string(textBytes))
					}
				} else {
					// 单字节字体（Symbol 等），逐字节处理
					var sb strings.Builder
					for _, b := range textBytes {
						ch := symbolToUnicode(b)
						if ch == '&' {
							sb.WriteString("&amp;")
						} else if ch == '<' {
							sb.WriteString("&lt;")
						} else if ch == '>' {
							sb.WriteString("&gt;")
						} else {
							sb.WriteRune(ch)
						}
					}
					textContent = sb.String()
				}
				elements = append(elements, enum.SvgElement{
					Tag: "text",
					Attrs: map[string]string{
						"x":         "0",
						"y":         "0",
						"style":     fmt.Sprintf("font-family:%s; font-style:%s; font-weight:normal; font-size:%.6f; fill:black", fontFamily, fontStyle, scaledFontSize),
						"transform": fmt.Sprintf("matrix(1.000000 -0.000000 0.000000 1.000000 %.6f %.6f)", charX, charY),
					},
					AttrOrder: []string{"x", "y", "style", "transform"},
					Inner:     textContent,
					ZOrder:    zOrder,
				})
				zOrder++
			} else {
				// Has Dx array: position each character individually
				tx := textX
				if state.CurrentCharset == 0x86 {
					// GBK：先解码为 UTF-8 字符串，再按 rune 逐字定位
					decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(textBytes)
					if err != nil {
						decoded = textBytes
					}
					runes := []rune(string(decoded))
					for i, ch := range runes {
						charX := float64(tx) * scaleX
						charY := float64(textY) * scaleY
						charStr := escapeXml(string(ch))
						elements = append(elements, enum.SvgElement{
							Tag: "text",
							Attrs: map[string]string{
								"x":         "0",
								"y":         "0",
								"style":     fmt.Sprintf("font-family:%s; font-style:%s; font-weight:normal; font-size:%.6f; fill:black", fontFamily, fontStyle, scaledFontSize),
								"transform": fmt.Sprintf("matrix(1.000000 -0.000000 0.000000 1.000000 %.6f %.6f)", charX, charY),
							},
							AttrOrder: []string{"x", "y", "style", "transform"},
							Inner:     charStr,
							ZOrder:    zOrder,
						})
						zOrder++
						if i < len(dxArray) {
							tx += dxArray[i]
						}
					}
				} else {
					// 单字节字体（Symbol 等），逐字节处理
					for i, b := range textBytes {
						charX := float64(tx) * scaleX
						charY := float64(textY) * scaleY
						ch := symbolToUnicode(b)
						charStr := string(ch)
						if ch == '&' {
							charStr = "&amp;"
						} else if ch == '<' {
							charStr = "&lt;"
						} else if ch == '>' {
							charStr = "&gt;"
						}
						elements = append(elements, enum.SvgElement{
							Tag: "text",
							Attrs: map[string]string{
								"x":         "0",
								"y":         "0",
								"style":     fmt.Sprintf("font-family:%s; font-style:%s; font-weight:normal; font-size:%.6f; fill:black", fontFamily, fontStyle, scaledFontSize),
								"transform": fmt.Sprintf("matrix(1.000000 -0.000000 0.000000 1.000000 %.6f %.6f)", charX, charY),
							},
							AttrOrder: []string{"x", "y", "style", "transform"},
							Inner:     charStr,
							ZOrder:    zOrder,
						})
						zOrder++
						if i < len(dxArray) {
							tx += dxArray[i]
						}
					}
				}
			}

			// Update CurrentPos after text output
			if len(dxArray) > 0 {
				// Dx array: advance by total of Dx values
				totalDx := int32(0)
				for _, dx := range dxArray {
					totalDx += dx
				}
				state.CurrentPos.X = textX + totalDx
				state.CurrentPos.Y = textY
			} else {
				// No Dx: estimate advance based on character count and font size
				state.CurrentPos.X = textX + int32(float64(strLen)*fontSize*0.6)
				state.CurrentPos.Y = textY
			}

		case enum.META_SETTEXTCOLOR:
			if len(params) >= 4 {
				val := binary.LittleEndian.Uint32(params[0:4])
				state.TextColor = enum.PenColor{uint8(val & 0xFF), uint8((val >> 8) & 0xFF), uint8((val >> 16) & 0xFF)}
			}

		case enum.META_CREATEFONTINDIRECT:
			if len(params) >= 18 {
				height := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				italic := params[10] != 0

				faceName := ""
				isGBK := false
				if len(params) > 18 {
					nameBytes := params[18:]
					nullIdx := len(nameBytes)
					for i, b := range nameBytes {
						if b == 0 {
							nullIdx = i
							break
						}
					}
					rawName := nameBytes[:nullIdx]
					// 检测字体名是否包含高字节（非 ASCII），判断为 GBK 编码
					hasHighByte := false
					for _, b := range rawName {
						if b >= 0x80 {
							hasHighByte = true
							break
						}
					}
					if hasHighByte {
						decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(rawName)
						if err == nil {
							faceName = string(decoded)
							isGBK = true
						} else {
							faceName = string(rawName)
						}
					} else {
						faceName = string(rawName)
					}
				}

				fontSize := math.Abs(float64(height))

				if faceName == "Times New Roman" {
					faceName = "Times"
				}

				charset := byte(0x00)
				if isGBK {
					charset = 0x86 // 内部标记为 GBK
				}

				slot := firstNullSlot(state.Objects)
				state.Objects[slot] = enum.GdiObject{
					Type:     "font",
					FontName: faceName,
					FontSize: fontSize,
					Italic:   italic,
					Charset:  charset,
				}
			}

		case enum.META_CREATEPENINDIRECT:
			if len(params) >= 8 {
				x := int32(binary.LittleEndian.Uint16(params[2:4]))
				y := int32(binary.LittleEndian.Uint16(params[4:6]))
				color := binary.LittleEndian.Uint32(params[6:10])
				slot := firstNullSlot(state.Objects)
				state.Objects[slot] = enum.GdiObject{Type: "pen"}
				state.PenWidth = x
				state.PenHeight = y
				state.PenColor = enum.PenColor{uint8(color & 0xFF), uint8((color >> 8) & 0xFF), uint8((color >> 16) & 0xFF)}
			}

		case enum.META_CREATEBRUSHINDIRECT:
			if len(params) >= 6 {
				color := binary.LittleEndian.Uint32(params[2:6])
				slot := firstNullSlot(state.Objects)
				state.Objects[slot] = enum.GdiObject{Type: "brush"}
				state.BrushColor = enum.PenColor{uint8(color & 0xFF), uint8((color >> 8) & 0xFF), uint8((color >> 16) & 0xFF)}
			}

		case enum.META_SELECTOBJECT:
			if len(params) >= 2 {
				objIdx := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				if obj, ok := state.Objects[objIdx]; ok {
					switch obj.Type {
					case "font":
						state.CurrentFont = obj.FontName
						state.CurrentFontSz = obj.FontSize
						state.CurrentItalic = obj.Italic
						state.CurrentCharset = obj.Charset
					}
				}
			}

		case enum.META_DELETEOBJECT:
			if len(params) >= 2 {
				objIdx := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				delete(state.Objects, objIdx)
			}

		case enum.META_ESCAPE:
			// Skip

		case enum.META_SETBKMODE:
			if len(params) >= 2 {
				state.BkMode = int32(binary.LittleEndian.Uint16(params[0:2]))
			}

		case enum.META_SETPOLYFILLMODE:
			if len(params) >= 2 {
				state.PolyFillMode = int32(binary.LittleEndian.Uint16(params[0:2]))
			}

		case enum.META_SETMAPMODE:
			if len(params) >= 2 {
				state.MapMode = int32(binary.LittleEndian.Uint16(params[0:2]))
			}

		case enum.META_SETWINDOWORG:
			if len(params) >= 4 {
				// WMF spec: y first, then x
				y := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				x := int32(int16(binary.LittleEndian.Uint16(params[2:4])))
				state.WinOrg = enum.Point{x, y}
			}

		case enum.META_SETWINDOWEXT:
			if len(params) >= 4 {
				// WMF spec: yExt first, then xExt
				yExt := int32(binary.LittleEndian.Uint16(params[0:2]))
				xExt := int32(binary.LittleEndian.Uint16(params[2:4]))
				state.WinExt = enum.Point{xExt, yExt}
			}

		case enum.META_SETVIEWPORTORG:
			if len(params) >= 4 {
				// WMF spec: y first, then x
				y := int32(int16(binary.LittleEndian.Uint16(params[0:2])))
				x := int32(int16(binary.LittleEndian.Uint16(params[2:4])))
				state.ViewportOrg = enum.Point{x, y}
			}

		case enum.META_SETVIEWPORTEXT:
			if len(params) >= 4 {
				// WMF spec: yExt first, then xExt
				yExt := int32(binary.LittleEndian.Uint16(params[0:2]))
				xExt := int32(binary.LittleEndian.Uint16(params[2:4]))
				state.ViewportExt = enum.Point{xExt, yExt}
			}

		case enum.META_SETPENWIDTH:
			if len(params) >= 2 {
				state.PenWidth = int32(binary.LittleEndian.Uint16(params[0:2]))
			}
		}

		offset = recEnd
	}

	return buildSvg(elements, imgW, imgH), nil
}

func buildSvg(elements []enum.SvgElement, w, h int32) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" standalone="no"?>
`)
	sb.WriteString(fmt.Sprintf(`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 20001102//EN"
"http://www.w3.org/TR/2000/CR-SVG-20001102/DTD/svg-20001102.dtd">
<svg width="%d" height="%d"
	xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd">
<desc>wmf2svg</desc>
`, w, h))

	for _, e := range elements {
		sb.WriteString("<" + e.Tag)
		for _, k := range e.AttrOrder {
			if v, ok := e.Attrs[k]; ok {
				sb.WriteString(fmt.Sprintf(` %s="%s"`, k, v))
			}
		}
		if len(e.AttrOrder) < len(e.Attrs) {
			orderedSet := make(map[string]bool)
			for _, k := range e.AttrOrder {
				orderedSet[k] = true
			}
			for k, v := range e.Attrs {
				if !orderedSet[k] {
					sb.WriteString(fmt.Sprintf(` %s="%s"`, k, v))
				}
			}
		}
		if e.Inner != "" {
			sb.WriteString("\n\t>" + e.Inner + "</" + e.Tag + ">\n")
		} else {
			sb.WriteString(" />\n")
		}
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

func parsePoints(data []byte, numPts int) []enum.Point {
	var pts []enum.Point
	for i := 0; i < numPts && i*4+4 <= len(data); i++ {
		x := int32(int16(binary.LittleEndian.Uint16(data[i*4 : i*4+2])))
		y := int32(int16(binary.LittleEndian.Uint16(data[i*4+2 : i*4+4])))
		pts = append(pts, enum.Point{x, y})
	}
	return pts
}

func colorStr(c enum.PenColor) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func escapeXml(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func abs32(a int32) int32 {
	if a < 0 {
		return -a
	}
	return a
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func firstNullSlot(objects map[int32]enum.GdiObject) int32 {
	for i := int32(0); ; i++ {
		if _, exists := objects[i]; !exists {
			return i
		}
	}
}

func symbolToUnicode(b byte) rune {
	switch b {
	case 0xB4:
		return 0x00B7
	case 0x2D:
		return 0x2212
	case 0xB9:
		return 0x2265
	case 0xA3:
		return 0x2264
	case 0xB2:
		return 0x2282
	case 0xC9:
		return 0x2286
	default:
		return rune(b)
	}
}
