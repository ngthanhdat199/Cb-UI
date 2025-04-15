package main

import (
	"fmt"
	"image/color" // Needed for chart data color
	"math"        // Needed for chart layout calculations

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Global reference to the main window
var mainWindow fyne.Window

// --- Custom Chart Widget ---
// (Code moved from the separate chart file)

// connectionStatsChart implements fyne.Widget
type connectionStatsChart struct {
	widget.BaseWidget
	title     string
	data      []float64 // Data points (representing KB/s or similar)
	maxY      float64   // Max value for the Y-axis scale (e.g., 11.2)
	yLabelMax string    // Label for the top Y-axis (e.g., "11.2KB/s")
	yLabelMin string    // Label for the bottom Y-axis (e.g., "0B/s")
}

// newConnectionStatsChart creates a new instance of the chart widget
func newConnectionStatsChart(title string, data []float64, maxY float64, yLabelMax, yLabelMin string) *connectionStatsChart {
	c := &connectionStatsChart{
		title:     title,
		data:      data,
		maxY:      maxY,
		yLabelMax: yLabelMax,
		yLabelMin: yLabelMin,
	}
	c.ExtendBaseWidget(c) // Important for custom widgets
	return c
}

// CreateRenderer is required by fyne.Widget
func (c *connectionStatsChart) CreateRenderer() fyne.WidgetRenderer {
	// Specific yellow/gold data color from the image
	dataColor := color.NRGBA{R: 0xff, G: 0xd7, B: 0x00, A: 0xff} // Yellow/Gold

	// Use theme color for text elements for adaptability
	titleText := canvas.NewText(c.title, theme.ForegroundColor())
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	// Adjust title size relative to standard text size if needed
	titleText.TextSize = theme.TextSize() * 1.4 // Make title reasonably large

	// Use a less prominent color for axis labels and lines (theme aware)
	labelColor := theme.ForegroundColor() // Use standard text color for labels too for visibility
	lineColor := theme.DisabledColor()    // Use disabled color for grid lines

	r := &connectionStatsChartRenderer{
		chart:     c,
		titleText: titleText,
		yLabelMax: canvas.NewText(c.yLabelMax, labelColor),
		yLabelMin: canvas.NewText(c.yLabelMin, labelColor),
		topLine:   canvas.NewLine(lineColor),
		midLine:   canvas.NewLine(lineColor),
		botLine:   canvas.NewLine(lineColor),
		dataLines: []fyne.CanvasObject{},
		dataColor: dataColor,
	}
	// Adjust label sizes if desired
	r.yLabelMax.TextSize = theme.TextSize() * 1.1
	r.yLabelMin.TextSize = theme.TextSize() * 1.1

	r.updateDataLines() // Initial creation of data lines
	r.Refresh()         // Initial setup
	return r
}

// --- Custom Chart Renderer ---

// connectionStatsChartRenderer implements fyne.WidgetRenderer
type connectionStatsChartRenderer struct {
	chart     *connectionStatsChart
	titleText *canvas.Text
	yLabelMax *canvas.Text
	yLabelMin *canvas.Text
	topLine   *canvas.Line
	midLine   *canvas.Line
	botLine   *canvas.Line
	dataLines []fyne.CanvasObject // Holds the vertical lines representing data
	dataColor color.Color
}

// Layout positions and sizes the elements of the chart
func (r *connectionStatsChartRenderer) Layout(size fyne.Size) {
	padding := theme.Padding()

	// --- Calculate available space, considering title and labels ---
	titleHeight := float32(0)
	// Only account for title height if title is not empty
	if r.chart.title != "" {
		titleHeight = r.titleText.MinSize().Height + padding
	}

	labelWidth := float32(math.Max(float64(r.yLabelMax.MinSize().Width), float64(r.yLabelMin.MinSize().Width))) + padding*1.5

	// --- Position Title ---
	if r.chart.title != "" {
		r.titleText.Move(fyne.NewPos(padding, padding/2))
		r.titleText.Show()
	} else {
		r.titleText.Hide() // Hide if title is empty
	}

	// --- Chart drawing area calculations ---
	chartAreaX := labelWidth
	chartAreaWidth := size.Width - labelWidth - padding // Padding on right too
	chartAreaY := titleHeight + r.yLabelMax.MinSize().Height/2
	chartAreaHeight := size.Height - chartAreaY - r.yLabelMin.MinSize().Height/2 - padding

	// Ensure calculated dimensions are not negative
	if chartAreaWidth < 0 {
		chartAreaWidth = 0
	}
	if chartAreaHeight < 0 {
		chartAreaHeight = 0
	}

	// --- Position Labels ---
	r.yLabelMax.Move(fyne.NewPos(padding, chartAreaY-r.yLabelMax.MinSize().Height/2))
	r.yLabelMin.Move(fyne.NewPos(padding, size.Height-padding-r.yLabelMin.MinSize().Height/2))

	// --- Position Grid Lines ---
	topY := chartAreaY
	botY := size.Height - padding - r.yLabelMin.MinSize().Height/2
	midY := topY + (botY-topY)/2
	lineEndX := chartAreaX + chartAreaWidth

	r.topLine.Position1 = fyne.NewPos(chartAreaX, topY)
	r.topLine.Position2 = fyne.NewPos(lineEndX, topY)
	r.midLine.Position1 = fyne.NewPos(chartAreaX, midY)
	r.midLine.Position2 = fyne.NewPos(lineEndX, midY)
	r.botLine.Position1 = fyne.NewPos(chartAreaX, botY)
	r.botLine.Position2 = fyne.NewPos(lineEndX, botY)

	// --- Position Data Lines ---
	if len(r.chart.data) == 0 || chartAreaWidth <= 0 || chartAreaHeight <= 0 || r.chart.maxY <= 0 {
		for _, line := range r.dataLines {
			line.Hide()
		}
		return
	} else {
		for _, line := range r.dataLines {
			line.Show()
		}
	}

	stepX := chartAreaWidth / float32(len(r.chart.data))
	scaleY := chartAreaHeight / float32(r.chart.maxY)

	for i, lineObj := range r.dataLines {
		if castLine, ok := lineObj.(*canvas.Line); ok {
			dataVal := r.chart.data[i]
			if dataVal < 0 {
				dataVal = 0
			}

			xPos := chartAreaX + float32(i)*stepX
			dataHeight := float32(dataVal) * scaleY
			if dataHeight > chartAreaHeight {
				dataHeight = chartAreaHeight
			}
			if dataHeight < 0 { // Should not happen with clamping above, but safety check
				dataHeight = 0
			}

			yPosData := botY - dataHeight

			castLine.Position1 = fyne.NewPos(xPos, botY)
			castLine.Position2 = fyne.NewPos(xPos, yPosData)
			castLine.StrokeWidth = float32(math.Max(1.0, float64(stepX*0.95)))
		}
	}
}

// MinSize calculates the minimum required size for the chart
func (r *connectionStatsChartRenderer) MinSize() fyne.Size {
	titleMinHeight := float32(0)
	titleMinWidth := float32(0)
	if r.chart.title != "" {
		titleMin := r.titleText.MinSize()
		titleMinHeight = titleMin.Height
		titleMinWidth = titleMin.Width
	}

	labelWidth := float32(math.Max(float64(r.yLabelMax.MinSize().Width), float64(r.yLabelMin.MinSize().Width)))
	minChartAreaHeight := theme.TextSize() * 3 // Minimum space for chart itself
	minChartAreaWidth := float32(50)            // Minimum pixel width for the data area

	minHeight := titleMinHeight + r.yLabelMax.MinSize().Height + r.yLabelMin.MinSize().Height + minChartAreaHeight + theme.Padding()*4
	minWidth := float32(math.Max(float64(titleMinWidth), float64(labelWidth+minChartAreaWidth))) + theme.Padding()*3

	return fyne.NewSize(minWidth, minHeight)
}

// Refresh updates the visual elements
func (r *connectionStatsChartRenderer) Refresh() {
	// Update colors based on theme potentially changing
	r.titleText.Color = theme.ForegroundColor()
	r.yLabelMax.Color = theme.ForegroundColor()
	r.yLabelMin.Color = theme.ForegroundColor()
	r.topLine.StrokeColor = theme.DisabledColor()
	r.midLine.StrokeColor = theme.DisabledColor()
	r.botLine.StrokeColor = theme.DisabledColor()

	// Update Text Content
	r.titleText.Text = r.chart.title
	r.yLabelMax.Text = r.chart.yLabelMax
	r.yLabelMin.Text = r.chart.yLabelMin

	r.titleText.Refresh()
	r.yLabelMax.Refresh()
	r.yLabelMin.Refresh()

	r.updateDataLines() // Handles data line colors too

	r.topLine.Refresh()
	r.midLine.Refresh()
	r.botLine.Refresh()
	for _, line := range r.dataLines {
		line.Refresh()
	}

	r.Layout(r.chart.Size())
	canvas.Refresh(r.chart)
}

// updateDataLines ensures the number of line objects matches the data points
func (r *connectionStatsChartRenderer) updateDataLines() {
	currentLen := len(r.dataLines)
	targetLen := len(r.chart.data)

	if currentLen < targetLen {
		for i := currentLen; i < targetLen; i++ {
			line := canvas.NewLine(r.dataColor)
			line.StrokeWidth = 1
			r.dataLines = append(r.dataLines, line)
		}
	} else if currentLen > targetLen {
		r.dataLines = r.dataLines[:targetLen]
	}

	// Ensure colors are correct even if only length changed
	for _, obj := range r.dataLines {
		if line, ok := obj.(*canvas.Line); ok {
			line.StrokeColor = r.dataColor
		}
	}
}

// Objects returns all canvas objects that make up the widget
func (r *connectionStatsChartRenderer) Objects() []fyne.CanvasObject {
	objects := r.dataLines
	objects = append(objects, r.topLine, r.midLine, r.botLine)
	// Only add title if it's not empty
	if r.chart.title != "" {
		objects = append(objects, r.titleText)
	}
	objects = append(objects, r.yLabelMax, r.yLabelMin)
	return objects
}

// Destroy is called when the widget is removed
func (r *connectionStatsChartRenderer) Destroy() {}

// --- Main Application ---
func main() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme()) // Keep dark theme

	mainWindow = a.NewWindow("Fyne App")

	loginContent := createLoginScreen()
	mainWindow.SetContent(loginContent)

	mainWindow.Resize(fyne.NewSize(800, 600))
	mainWindow.CenterOnScreen()
	mainWindow.Show()

	a.Run()
}

// --- Login Screen ---
// (No changes needed here)
func createLoginScreen() fyne.CanvasObject {
	userNameLabel := widget.NewLabel("John Doe")
	topContent := container.NewHBox(layout.NewSpacer(), userNameLabel)

	titleLabel := widget.NewLabelWithStyle(
		"Connect Bridge",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	signInButton := widget.NewButton("[Signin]", func() {
		fmt.Println("Sign in clicked!")
		loggedInContent := createLoggedInScreen()
		mainWindow.SetContent(loggedInContent)
	})

	centerBox := container.NewVBox(
		layout.NewSpacer(),
		titleLabel,
		signInButton,
		layout.NewSpacer(),
	)
	centerContent := container.NewCenter(centerBox)

	loginLayout := container.NewBorder(
		container.NewPadded(topContent),
		nil, nil, nil,
		centerContent,
	)

	return loginLayout
}

// --- Logged In Screen (Gateway List) ---
// (No changes needed here)
func createLoggedInScreen() fyne.CanvasObject {
	userName := widget.NewLabel("Vinh Nguyen")
	userEmail := widget.NewLabel("vinh@gmail.com")
	userInfo := container.NewVBox(userName, userEmail)
	topBar := container.NewHBox(layout.NewSpacer(), userInfo)

	quickConnectButton := widget.NewButton("[Quick Connect]", func() {
		fmt.Println("Quick Connect clicked")
		connectedContent := createConnectedScreen("Quick Connect Gateway", "auto-region", "your-device (IP unknown)")
		mainWindow.SetContent(connectedContent)
	})

	gatewaysLabel := widget.NewLabelWithStyle("Gateways", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	recentConnectionsLabel := widget.NewLabel("Recent connections")
	tabsHeader := container.NewHBox(gatewaysLabel, layout.NewSpacer(), recentConnectionsLabel)

	gatewayList := container.NewVBox()
	gateways := []struct {
		name   string
		region string
	}{
		{"Ho Chi Minh Office", "ap-southeast-3"},
		{"Thailand Office", "ap-southeast-2"},
		{"Germany Office", "europe-3"},
	}

	for _, gw := range gateways {
		gwName := gw.name
		gwRegion := gw.region
		nameLabel := widget.NewLabel(gwName)
		regionLabel := widget.NewLabel(gwRegion)
		infoVBox := container.NewVBox(nameLabel, regionLabel)
		connectButton := widget.NewButton("[Connect]", func() {
			fmt.Printf("Connect clicked for: %s\n", gwName)
			deviceName := "john-laptop (100.100.24.3)"
			connectedContent := createConnectedScreen(gwName, gwRegion, deviceName)
			mainWindow.SetContent(connectedContent)
		})
		row := container.NewHBox(infoVBox, layout.NewSpacer(), connectButton)
		gatewayList.Add(row)
		gatewayList.Add(widget.NewSeparator())
	}

	leftSideContent := container.NewVBox(
		quickConnectButton,
		widget.NewSeparator(),
		tabsHeader,
		widget.NewSeparator(),
		gatewayList,
		layout.NewSpacer(),
	)

	mapPlaceholder := widget.NewLabel("the map for available gateways")
	rightSideContent := container.NewCenter(mapPlaceholder)

	centerSplit := container.NewHSplit(
		container.NewPadded(leftSideContent),
		container.NewPadded(rightSideContent),
	)
	centerSplit.Offset = 0.4

	loggedInLayout := container.NewBorder(
		container.NewPadded(topBar),
		nil, nil, nil,
		centerSplit,
	)

	return loggedInLayout
}

// --- Connected Screen (Connection Details & Stats) ---
// *** MODIFIED TO USE CUSTOM CHART ***
func createConnectedScreen(gatewayName, gatewayRegion, deviceName string) fyne.CanvasObject {

	// --- Top Right User Info & Logout ---
	userNameLabel := widget.NewLabel("Vinh Nguyen")
	logoutButton := widget.NewButton("[Logout]", func() {
		fmt.Println("Logout clicked")
		loginContent := createLoginScreen()
		mainWindow.SetContent(loginContent)
	})
	topBar := container.NewHBox(layout.NewSpacer(), userNameLabel, logoutButton)

	// --- Center Content Area ---

	// Connection Info Section
	statusLabel := widget.NewLabelWithStyle("Connected", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	gwNameLabel := widget.NewLabel(gatewayName)
	gwRegionLabel := widget.NewLabel(gatewayRegion)
	gatewayDetailsVBox := container.NewVBox(gwNameLabel, gwRegionLabel)
	deviceLabel := widget.NewLabel(deviceName)
	connectionDetailsHBox := container.NewHBox(gatewayDetailsVBox, layout.NewSpacer(), deviceLabel)
	disconnectButton := widget.NewButton("[Disconnect]", func() {
		fmt.Println("Disconnect clicked")
		loggedInContent := createLoggedInScreen()
		mainWindow.SetContent(loggedInContent)
	})

	connectionInfoSection := container.NewVBox(
		statusLabel,
		connectionDetailsHBox,
		disconnectButton,
	)

	// --- Instantiate the Custom Chart ---
	// Sample data resembling the shape in the image
	chartData := []float64{
		0.1, 0.1, 0.2, 0.1, 0.2, 0.3, 0.2, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1,
		0.1, 0.1, 0.2, 0.3, 0.5, 0.8, 1.2, 1.8, 2.5, 3.0, 2.2, 1.5, 0.8, 0.5, 0.3,
		0.2, 0.2, 0.2, 0.3, 0.4, 0.8, 1.5, 2.5, 4.0, 6.0, 8.5, 10.0, 11.0, 10.8, 9.5,
		8.0, 6.5, 5.5, 4.8, 4.0, 3.5, 3.0, 2.6, 2.2, 1.8, 1.5, 1.2, 1.0, 0.8, 0.6,
		0.5, 0.4, 0.3, 0.2, 0.2, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1,
	}
	maxYValue := 11.2

	// Create the chart widget instance using the code moved into this file
	// Use the exact title ("Connction Stats") and labels from the image
	chartWidget := newConnectionStatsChart(
		"Connction Stats", // Title displayed *by the chart widget itself*
		chartData,
		maxYValue,
		"11.2KB/s", // Top Y-axis label
		"0B/s",     // Bottom Y-axis label
	)
	// Give the chart a minimum size within the layout
	// We use a Max container to allow it to expand, but the chart's MinSize provides the base.
	chartContainer := container.NewMax(chartWidget) // Wrap chart in Max for better sizing control


	// Bytes In/Out Section
	bytesInIcon := widget.NewIcon(theme.MoveDownIcon()) // More appropriate icon?
	bytesInLabel := widget.NewLabel("BYTES IN\n0 KB/S")   // Multi-line
	bytesInSection := container.NewHBox(bytesInIcon, bytesInLabel)

	bytesOutIcon := widget.NewIcon(theme.MoveUpIcon()) // More appropriate icon?
	bytesOutLabel := widget.NewLabel("BYTES OUT\n0 KB/S") // Multi-line
	bytesOutSection := container.NewHBox(bytesOutIcon, bytesOutLabel)

	bytesHBox := container.NewHBox(bytesInSection, layout.NewSpacer(), bytesOutSection)

	// --- Assemble Center VBox ---
	centerContent := container.NewVBox(
		connectionInfoSection,
		widget.NewSeparator(),
		// Removed separate statsTitle and currentSpeedLabel
		chartContainer, // Add the actual chart widget (in its container)
		widget.NewSeparator(),
		bytesHBox,
		layout.NewSpacer(), // Push content up
	)

	// --- Assemble Connected Layout ---
	connectedLayout := container.NewBorder(
		container.NewPadded(topBar),
		nil, nil, nil,
		container.NewPadded(centerContent), // Pad the central content
	)

	return connectedLayout
}