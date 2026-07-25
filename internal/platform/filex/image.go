package filex

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"golang.org/x/image/webp"
)

func decodeImageConfig(data []byte) (image.Config, string, error) {
	reader := bytes.NewReader(data)

	config, format, err := image.DecodeConfig(reader)
	if err == nil {
		return config, format, nil
	}

	reader.Seek(0, io.SeekStart)
	config, err = webp.DecodeConfig(reader)
	if err == nil {
		return config, "webp", nil
	}

	if heifMime := detectHEIF(data); heifMime != "" {
		formatName := "heif"
		if heifMime == "image/heic" {
			formatName = "heic"
		}
		if w, h, ok := parseHEIFDimensions(data); ok {
			return image.Config{Width: w, Height: h}, formatName, nil
		}
		return image.Config{}, "", fmt.Errorf("failed to decode HEIF/HEIC image dimensions")
	}

	return image.Config{}, "", fmt.Errorf("failed to decode image config: unsupported format")
}

func detectHEIF(data []byte) string {
	if len(data) < 12 {
		return ""
	}
	if string(data[4:8]) != "ftyp" {
		return ""
	}
	brand := string(data[8:12])
	switch brand {
	case "heic", "heix", "hevc", "hevx", "heim", "heis":
		return "image/heic"
	case "mif1", "msf1":
		return "image/heif"
	default:
		return ""
	}
}

func parseHEIFDimensions(data []byte) (int, int, bool) {
	size := len(data)
	if size < 12 {
		return 0, 0, false
	}

	offset := 0
	for offset+8 <= size {
		boxSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		boxType := string(data[offset+4 : offset+8])
		headerLen := 8

		if boxSize == 1 {
			if offset+16 > size {
				break
			}
			boxSize = int(binary.BigEndian.Uint64(data[offset+8 : offset+16]))
			headerLen = 16
		} else if boxSize == 0 {
			boxSize = size - offset
		}

		if boxSize < headerLen || offset+boxSize > size {
			break
		}

		if boxType == "meta" {
			metaData := data[offset+headerLen : offset+boxSize]
			if len(metaData) < 4 {
				return 0, 0, false
			}
			return findISPE(metaData[4:])
		}
		offset += boxSize
	}
	return 0, 0, false
}

func findISPE(data []byte) (int, int, bool) {
	offset := 0
	size := len(data)
	for offset+8 <= size {
		boxSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		boxType := string(data[offset+4 : offset+8])
		if boxSize < 8 || offset+boxSize > size {
			break
		}
		content := data[offset+8 : offset+boxSize]
		switch boxType {
		case "iprp", "ipco":
			if w, h, ok := findISPE(content); ok {
				return w, h, true
			}
		case "ispe":
			if len(content) >= 12 {
				w := int(binary.BigEndian.Uint32(content[4:8]))
				h := int(binary.BigEndian.Uint32(content[8:12]))
				if w > 0 && h > 0 {
					return w, h, true
				}
			}
		}
		offset += boxSize
	}
	return 0, 0, false
}

// DetectHEIF reports whether the payload is a HEIF/HEIC image.
func DetectHEIF(data []byte) string {
	return detectHEIF(data)
}

// ParseHEIFDimensions extracts image dimensions from a HEIF/HEIC payload.
func ParseHEIFDimensions(data []byte) (int, int, bool) {
	return parseHEIFDimensions(data)
}
