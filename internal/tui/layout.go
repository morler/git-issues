package tui

import "time"

// Layout constants. They name the magic numbers used while rendering the
// board and the detail view.
const (
	// boardStatusHeight is the space below the columns reserved for the status bar.
	boardStatusHeight = 3

	// columnChromeWidth is the horizontal border of a column.
	columnChromeWidth = 2

	// columnChromeHeight is the header line plus the vertical border of a column.
	columnChromeHeight = 3

	// cardRowHeight estimates the rendered height of one card (border + lines).
	cardRowHeight = 4

	// cardBorderWidth is the horizontal card border.
	cardBorderWidth = 2

	// cardTextInset is the horizontal space taken by the card border and padding.
	cardTextInset = 4

	// cardIDWidth is the width of the "#123" prefix on a card.
	cardIDWidth = 4

	// detailContentInset is the horizontal space taken by the detail border and padding.
	detailContentInset = 6

	// minDetailContentWidth keeps detail content readable in narrow terminals.
	minDetailContentWidth = 20

	// detailViewportInset is the vertical space taken by the detail frame and status bar.
	detailViewportInset = 6

	// minDetailViewportHeight keeps the detail viewport usable in short terminals.
	minDetailViewportHeight = 5

	// detailFrameWidthMargin keeps the framed detail view inside the terminal.
	detailFrameWidthMargin = 4

	// watchDebounce coalesces rapid filesystem events into one reload.
	watchDebounce = 200 * time.Millisecond
)
