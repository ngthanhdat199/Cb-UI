package main

// import (
// 	"image/color"
// 	"math"

// 	"fyne.io/fyne/v2"
// 	"fyne.io/fyne/v2/app"
// 	"fyne.io/fyne/v2/canvas"
// 	"fyne.io/fyne/v2/theme"
// 	"fyne.io/fyne/v2/widget"
// )

// // --- Custom Chart Widget ---

// // connectionStatsChart implements fyne.Widget
// type connectionStatsChart struct {
// 	widget.BaseWidget
// 	title     string
// 	data      []float64 // Data points (representing KB/s or similar)
// 	maxY      float64   // Max value for the Y-axis scale (e.g., 11.2)
// 	yLabelMax string    // Label for the top Y-axis (e.g., "11.2KB/s")
// 	yLabelMin string    // Label for the bottom Y-axis (e.g., "0B/s")
// }

// // newConnectionStatsChart creates a new instance of the chart widget
// func newConnectionStatsChart(title string, data []float64, maxY float64, yLabelMax, yLabelMin string) *connectionStatsChart {
// 	c := &connectionStatsChart{
// 		title:     title,
// 		data:      data,
// 		maxY:      maxY,
// 		yLabelMax: yLabelMax,
// 		yLabelMin: yLabelMin,
// 	}
// 	c.ExtendBaseWidget(c) // Important for custom widgets
// 	return c
// }

// // CreateRenderer is required by fyne.Widget
// func (c *connectionStatsChart) CreateRenderer() fyne.WidgetRenderer {
// 	// Use NRGBA for the specific yellow/gold color
// 	// You might need to adjust this based on the exact color you want
// 	// This is a bright yellow/gold similar to the spike
// 	dataColor := color.NRGBA{R: 0xff, G: 0xd7, B: 0x00, A: 0xff}
// 	// A slightly darker orange/yellow potentially seen at the base (optional)
// 	// dataColorBase := color.NRGBA{R: 0xff, G: 0xa5, B: 0x00, A: 0xff}

// 	titleText := canvas.NewText(c.title, color.Black)
// 	titleText.TextStyle = fyne.TextStyle{Bold: true}
// 	titleText.TextSize = theme.TextSize() * 1.5 // Scale text size for heading

// 	r := &connectionStatsChartRenderer{
// 		chart:     c,
// 		titleText: titleText,
// 		yLabelMax: canvas.NewText(c.yLabelMax, color.Gray{Y: 100}), // Slightly darker gray
// 		yLabelMin: canvas.NewText(c.yLabelMin, color.Gray{Y: 100}),
// 		topLine:   canvas.NewLine(color.Gray{Y: 210}), // Lighter gray lines
// 		midLine:   canvas.NewLine(color.Gray{Y: 210}),
// 		botLine:   canvas.NewLine(color.Gray{Y: 210}),
// 		dataLines: []fyne.CanvasObject{}, // We'll use vertical lines to simulate area
// 		dataColor: dataColor,
// 	}
// 	r.yLabelMax.TextSize = theme.TextSize() * 1.1 // Make labels a bit larger
// 	r.yLabelMin.TextSize = theme.TextSize() * 1.1

// 	r.updateDataLines() // Initial creation of data lines
// 	r.Refresh()         // Initial setup
// 	return r
// }

// // --- Custom Chart Renderer ---

// // connectionStatsChartRenderer implements fyne.WidgetRenderer
// type connectionStatsChartRenderer struct {
// 	chart     *connectionStatsChart
// 	titleText *canvas.Text
// 	yLabelMax *canvas.Text
// 	yLabelMin *canvas.Text
// 	topLine   *canvas.Line
// 	midLine   *canvas.Line
// 	botLine   *canvas.Line
// 	dataLines []fyne.CanvasObject // Holds the vertical lines representing data
// 	dataColor color.Color
// }

// // Layout positions and sizes the elements of the chart
// func (r *connectionStatsChartRenderer) Layout(size fyne.Size) {
// 	padding := theme.Padding()
// 	titleHeight := r.titleText.MinSize().Height + padding
// 	labelWidth := float32(math.Max(float64(r.yLabelMax.MinSize().Width), float64(r.yLabelMin.MinSize().Width))) + padding*1.5 // More space for labels

// 	// Position Title
// 	r.titleText.Move(fyne.NewPos(padding, padding/2)) // Position title top-left with padding

// 	// Chart drawing area calculations
// 	chartAreaX := labelWidth
// 	chartAreaWidth := size.Width - labelWidth - padding                                    // Padding on right too
// 	chartAreaY := titleHeight + r.yLabelMax.MinSize().Height/2                             // Start below title, align line with middle of top label
// 	chartAreaHeight := size.Height - chartAreaY - r.yLabelMin.MinSize().Height/2 - padding // Padding at bottom

// 	// Position Labels
// 	// Align label vertical center with the corresponding line
// 	r.yLabelMax.Move(fyne.NewPos(padding, chartAreaY-r.yLabelMax.MinSize().Height/2))
// 	r.yLabelMin.Move(fyne.NewPos(padding, size.Height-padding-r.yLabelMin.MinSize().Height/2))

// 	// Position Grid Lines
// 	topY := chartAreaY
// 	botY := size.Height - padding - r.yLabelMin.MinSize().Height/2
// 	midY := topY + (botY-topY)/2

// 	// Ensure lines don't extend beyond the chart area width
// 	lineEndX := chartAreaX + chartAreaWidth

// 	r.topLine.Position1 = fyne.NewPos(chartAreaX, topY)
// 	r.topLine.Position2 = fyne.NewPos(lineEndX, topY)
// 	r.midLine.Position1 = fyne.NewPos(chartAreaX, midY)
// 	r.midLine.Position2 = fyne.NewPos(lineEndX, midY)
// 	r.botLine.Position1 = fyne.NewPos(chartAreaX, botY)
// 	r.botLine.Position2 = fyne.NewPos(lineEndX, botY)

// 	// Position Data Lines (vertical lines to simulate area)
// 	if len(r.chart.data) == 0 || chartAreaWidth <= 0 || chartAreaHeight <= 0 || r.chart.maxY <= 0 {
// 		// Hide data lines if invalid state
// 		for _, line := range r.dataLines {
// 			line.Hide()
// 		}
// 		return // Nothing to draw or invalid scale
// 	} else {
// 		for _, line := range r.dataLines {
// 			line.Show()
// 		}
// 	}

// 	stepX := chartAreaWidth / float32(len(r.chart.data))
// 	// Ensure scaleY is valid even if maxY is very small
// 	scaleY := chartAreaHeight / float32(r.chart.maxY)

// 	for i, lineObj := range r.dataLines {
// 		if castLine, ok := lineObj.(*canvas.Line); ok {
// 			dataVal := r.chart.data[i]
// 			if dataVal < 0 {
// 				dataVal = 0
// 			} // Don't draw below zero line

// 			xPos := chartAreaX + float32(i)*stepX
// 			// Calculate height based on data, clamping to chart area
// 			dataHeight := float32(dataVal) * scaleY
// 			if dataHeight > chartAreaHeight {
// 				dataHeight = chartAreaHeight
// 			}

// 			yPosData := botY - dataHeight // Y position for the top of the data line

// 			castLine.Position1 = fyne.NewPos(xPos, botY)     // Start at bottom line
// 			castLine.Position2 = fyne.NewPos(xPos, yPosData) // End at data value height
// 			// Make lines thick enough to mostly fill space, minimum 1 pixel wide
// 			castLine.StrokeWidth = float32(math.Max(1.0, float64(stepX*0.95))) // Adjust factor (0.95) for fill density
// 		}
// 	}
// }

// // MinSize calculates the minimum required size for the chart
// func (r *connectionStatsChartRenderer) MinSize() fyne.Size {
// 	titleMin := r.titleText.MinSize()
// 	labelWidth := float32(math.Max(float64(r.yLabelMax.MinSize().Width), float64(r.yLabelMin.MinSize().Width)))
// 	minHeight := titleMin.Height + r.yLabelMax.MinSize().Height + r.yLabelMin.MinSize().Height + theme.Padding()*4 // Vertical space for title, labels, chart
// 	minWidth := float32(math.Max(float64(titleMin.Width), float64(labelWidth+50))) + theme.Padding()*3             // Ensure space for title, labels, and some chart width (50px minimum chart width)
// 	return fyne.NewSize(float32(minWidth), float32(minHeight))
// }

// // Refresh updates the visual elements
// func (r *connectionStatsChartRenderer) Refresh() {
// 	r.titleText.Text = r.chart.title
// 	r.titleText.Refresh()

// 	r.yLabelMax.Text = r.chart.yLabelMax
// 	r.yLabelMin.Text = r.chart.yLabelMin
// 	r.yLabelMax.Refresh()
// 	r.yLabelMin.Refresh()

// 	// Ensure data lines match data array length
// 	r.updateDataLines()

// 	r.topLine.Refresh()
// 	r.midLine.Refresh()
// 	r.botLine.Refresh()
// 	for _, line := range r.dataLines {
// 		line.Refresh()
// 	}

// 	// Trigger layout recalculation and redraw
// 	r.Layout(r.chart.Size())
// 	canvas.Refresh(r.chart)
// }

// // updateDataLines ensures the number of line objects matches the data points
// func (r *connectionStatsChartRenderer) updateDataLines() {
// 	currentLen := len(r.dataLines)
// 	targetLen := len(r.chart.data)

// 	if currentLen < targetLen { // Add new lines
// 		for i := currentLen; i < targetLen; i++ {
// 			line := canvas.NewLine(r.dataColor)
// 			line.StrokeWidth = 1 // Initial width, layout will adjust
// 			r.dataLines = append(r.dataLines, line)
// 		}
// 	} else if currentLen > targetLen { // Remove excess lines
// 		r.dataLines = r.dataLines[:targetLen]
// 	}

// 	// Update color for all lines (in case it needs changing)
// 	for _, obj := range r.dataLines {
// 		if line, ok := obj.(*canvas.Line); ok {
// 			line.StrokeColor = r.dataColor
// 		}
// 	}
// }

// // Objects returns all canvas objects that make up the widget
// func (r *connectionStatsChartRenderer) Objects() []fyne.CanvasObject {
// 	// Draw data lines first (bottom layer)
// 	objects := r.dataLines
// 	// Then grid lines
// 	objects = append(objects, r.topLine, r.midLine, r.botLine)
// 	// Then labels and title on top
// 	objects = append(objects, r.titleText, r.yLabelMax, r.yLabelMin)
// 	return objects
// }

// // Destroy is called when the widget is removed
// func (r *connectionStatsChartRenderer) Destroy() {}

// // --- Main Application ---

// func chart() {
// 	a := app.New()
// 	w := a.NewWindow("Connection Stats Chart Example")

// 	// Sample data resembling the shape in the image
// 	// Max value around 11.2 to match the label
// 	data := []float64{
// 		0.1, 0.1, 0.2, 0.1, 0.2, 0.3, 0.2, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, // Initial low level
// 		0.1, 0.1, 0.2, 0.3, 0.5, 0.8, 1.2, 1.8, 2.5, 3.0, 2.2, 1.5, 0.8, 0.5, 0.3, // Small rise & fall
// 		0.2, 0.2, 0.2, 0.3, 0.4, 0.8, 1.5, 2.5, 4.0, 6.0, 8.5, 10.0, 11.0, 10.8, 9.5, // Big spike
// 		8.0, 6.5, 5.5, 4.8, 4.0, 3.5, 3.0, 2.6, 2.2, 1.8, 1.5, 1.2, 1.0, 0.8, 0.6, // Tapering off
// 		0.5, 0.4, 0.3, 0.2, 0.2, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, // Back to low
// 	}
// 	maxYValue := 11.2 // Important: This should match the scale used for yLabelMax

// 	// Create the custom chart widget
// 	// Note the typo "Connction" to match the image exactly
// 	chartWidget := newConnectionStatsChart(
// 		"Connction Stats",
// 		data,
// 		maxYValue,
// 		"11.2KB/s",
// 		"0B/s",
// 	)

// 	// Set up layout - just the chart in this case
// 	w.SetContent(chartWidget)
// 	w.Resize(fyne.NewSize(450, 280)) // Adjust size for better viewing
// 	w.ShowAndRun()
// }
