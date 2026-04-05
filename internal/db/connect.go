package db

import (
	"unicode/utf8"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a MySQL connection via GORM.
func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

// cp1252ToLatin1 maps Windows-1252 specific codepoints (0x80-0x9F range)
// back to their original byte values. MySQL's "latin1" is actually CP1252.
var cp1252ToLatin1 = map[rune]byte{
	0x20AC: 0x80, // €
	0x201A: 0x82, // ‚
	0x0192: 0x83, // ƒ
	0x201E: 0x84, // „
	0x2026: 0x85, // …
	0x2020: 0x86, // †
	0x2021: 0x87, // ‡
	0x02C6: 0x88, // ˆ
	0x2030: 0x89, // ‰
	0x0160: 0x8A, // Š
	0x2039: 0x8B, // ‹
	0x0152: 0x8C, // Œ
	0x017D: 0x8E, // Ž
	0x2018: 0x91, // '
	0x2019: 0x92, // '
	0x201C: 0x93, // "
	0x201D: 0x94, // "
	0x2022: 0x95, // •
	0x2013: 0x96, // –
	0x2014: 0x97, // —
	0x02DC: 0x98, // ˜
	0x2122: 0x99, // ™
	0x0161: 0x9A, // š
	0x203A: 0x9B, // ›
	0x0153: 0x9C, // œ
	0x017E: 0x9E, // ž
	0x0178: 0x9F, // Ÿ
}

// FixUTF8 repairs double-encoded UTF-8 strings from MySQL INFORMATION_SCHEMA.
// MySQL 8.0's "latin1" is actually Windows-1252. When UTF-8 metadata is read
// through a UTF-8 connection, the bytes get double-encoded. This function
// reverses that by mapping each rune back to its CP1252 byte value and
// re-interpreting the result as UTF-8.
func FixUTF8(s string) string {
	bytes := make([]byte, 0, len(s))
	for _, r := range s {
		if r <= 0xFF {
			bytes = append(bytes, byte(r))
		} else if b, ok := cp1252ToLatin1[r]; ok {
			bytes = append(bytes, b)
		} else {
			return s // not double-encoded
		}
	}
	decoded := string(bytes)
	if utf8.ValidString(decoded) {
		return decoded
	}
	return s
}
