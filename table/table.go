package table

import (
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// New creates a new tablewriter.Table instance with suitable defaults.
func New() *tablewriter.Table {
	// Create a borderless, minimal table with space padding.
	rendition := tw.Rendition{
		Borders: tw.BorderNone,
		Settings: tw.Settings{
			Separators: tw.Separators{
				BetweenRows:    tw.Off,
				BetweenColumns: tw.Off,
			},
			Lines: tw.Lines{
				ShowTop:        tw.Off,
				ShowBottom:     tw.Off,
				ShowHeaderLine: tw.Off,
				ShowFooterLine: tw.Off,
			},
		},
	}

	table := tablewriter.NewTable(
		os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint(rendition)),
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{
					AutoWrap:   tw.WrapNone,
					AutoFormat: tw.On,
				},
				Alignment: tw.CellAlignment{
					Global: tw.AlignLeft,
				},
				Padding: tw.CellPadding{
					Global: tw.Padding{Left: "", Right: "   "},
				},
			},
			Row: tw.CellConfig{
				Formatting: tw.CellFormatting{
					AutoWrap: tw.WrapNone,
				},
				Alignment: tw.CellAlignment{
					Global: tw.AlignLeft,
				},
				Padding: tw.CellPadding{
					Global: tw.Padding{Left: "", Right: "   "},
				},
			},
		}),
	)

	return table
}
