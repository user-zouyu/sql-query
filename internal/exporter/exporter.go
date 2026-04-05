package exporter

import "sql-query/internal/parser"

// Exporter is the interface for all export formats.
type Exporter interface {
	Export(columns []*parser.Column, data [][]*string) error
}
