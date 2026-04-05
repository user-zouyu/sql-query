package parser

import (
	"regexp"
	"strings"
)

// ColumnMeta holds structured metadata parsed from a column alias.
type ColumnMeta struct {
	Tag    string            `json:"tag"`
	Params string            `json:"params,omitempty"`
	Args   map[string]string `json:"args,omitempty"`
}

// Column represents a parsed column definition.
type Column struct {
	RawName     string                 `json:"name"`
	DisplayName string                 `json:"displayName"`
	Meta        map[string]*ColumnMeta `json:"meta,omitempty"`
}

var metaRegex = regexp.MustCompile(`\[(\w+)(?:\(([^)]*)\))?\]`)

// ParseColumns parses metadata from raw column names.
func ParseColumns(rawColumns []string) []*Column {
	columns := make([]*Column, len(rawColumns))
	for i, raw := range rawColumns {
		columns[i] = parseColumn(raw)
	}
	return columns
}

func parseColumn(rawName string) *Column {
	col := &Column{
		RawName: rawName,
		Meta:    make(map[string]*ColumnMeta),
	}

	displayName := rawName
	matches := metaRegex.FindAllStringSubmatch(rawName, -1)

	for _, match := range matches {
		tag := match[1]
		params := ""
		if len(match) > 2 {
			params = match[2]
		}

		meta := &ColumnMeta{
			Tag:    tag,
			Params: params,
			Args:   parseArgs(tag, params),
		}
		col.Meta[tag] = meta

		displayName = strings.Replace(displayName, match[0], "", 1)
	}

	col.DisplayName = strings.TrimSpace(displayName)
	if col.DisplayName == "" {
		col.DisplayName = rawName
	}

	return col
}

func parseArgs(tag, params string) map[string]string {
	args := make(map[string]string)
	if params == "" {
		return args
	}

	switch tag {
	case "URL":
		parts := strings.Split(params, ",")
		if len(parts) >= 1 {
			args["expiry"] = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "D" {
			args["download"] = "true"
		}
	case "HTML":
		parts := strings.SplitN(params, ",", 2)
		if len(parts) >= 1 {
			args["type"] = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			interaction := strings.TrimSpace(parts[1])
			args["interaction"] = interaction
			if len(interaction) >= 2 && interaction[1] == ':' {
				args["interactionType"] = string(interaction[0])
				rest := ""
				if len(interaction) > 2 {
					rest = interaction[2:]
				}
				if idx := strings.Index(rest, "->"); idx != -1 {
					args["hint"] = strings.TrimSpace(rest[:idx])
					args["bindColumn"] = strings.TrimSpace(rest[idx+2:])
				} else {
					args["hint"] = strings.TrimSpace(rest)
					args["bindToSelf"] = "true"
				}
			}
		}
	case "H":
		args["height"] = strings.TrimSpace(params)
	}

	return args
}

// HasMeta checks whether the column has the given metadata tag.
func (c *Column) HasMeta(tag string) bool {
	_, ok := c.Meta[tag]
	return ok
}

// GetMeta returns the metadata for the given tag, or nil.
func (c *Column) GetMeta(tag string) *ColumnMeta {
	return c.Meta[tag]
}
