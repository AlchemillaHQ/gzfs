package gzfs

import (
	"crypto/sha1"
	"fmt"
	"strconv"
	"strings"
)

type OutputVersion struct {
	Command   string `json:"command"`
	VersMajor int    `json:"vers_major"`
	VersMinor int    `json:"vers_minor"`
}

type ZFSProperty struct {
	Value  string            `json:"value"`
	Source ZFSPropertySource `json:"source"`
}

type ZFSPropertySource struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

var (
	zdbArgs   = []string{"-C"}
	zpoolArgs = []string{"-p"}
	zfsArgs   = []string{"-p"}
)

func ParseSize(value string) uint64 {
	s := strings.TrimSpace(value)
	if s == "" || s == "-" {
		return 0
	}

	// Strip non-size suffixes early
	if strings.HasSuffix(s, "%") || strings.HasSuffix(s, "x") {
		return 0
	}

	// Find numeric + unit boundary
	var numPart string
	var unitPart string

	for i, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			numPart = s[:i]
			unitPart = s[i:]
			break
		}
	}

	if numPart == "" {
		numPart = s
	}

	val, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(strings.TrimSpace(unitPart))

	switch unit {
	case "B", "":
		return uint64(val)
	case "K", "KB":
		return uint64(val * 1024)
	case "M", "MB":
		return uint64(val * 1024 * 1024)
	case "G", "GB":
		return uint64(val * 1024 * 1024 * 1024)
	case "T", "TB":
		return uint64(val * 1024 * 1024 * 1024 * 1024)
	case "P", "PB":
		return uint64(val * 1024 * 1024 * 1024 * 1024 * 1024)
	default:
		// Unknown unit
		return 0
	}
}

func GenerateDeterministicUUID(seed string) string {
	// Here we use the RFC 4122 URL namespace:
	// 6ba7b811-9dad-11d1-80b4-00c04fd430c8
	namespace := [16]byte{
		0x6b, 0xa7, 0xb8, 0x11,
		0x9d, 0xad,
		0x11, 0xd1,
		0x80, 0xb4,
		0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8,
	}

	h := sha1.New()
	h.Write(namespace[:])
	h.Write([]byte(seed))
	sum := h.Sum(nil)

	var uuid [16]byte
	copy(uuid[:], sum[:16])

	// version 5
	uuid[6] = (uuid[6] & 0x0f) | 0x50
	// RFC 4122 variant
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:],
	)
}

func ParseUint64(value string) uint64 {
	v, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}

	return v
}

func ParseFloat64(value string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}

	return v
}

func ParseRatio(value string) float64 {
	v := strings.TrimSpace(value)
	v = strings.TrimSuffix(v, "x")

	if v == "" {
		return 0
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}

	return f
}

func ParseString(value string) string {
	v := strings.TrimSpace(value)
	if v == "-" {
		return ""
	}

	return v
}

func ParsePercentage(value string) float64 {
	v := strings.TrimSpace(value)
	v = strings.TrimSuffix(v, "%")

	if v == "" {
		return 0
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}

	return f
}
