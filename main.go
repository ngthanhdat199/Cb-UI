package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var mainWindow fyne.Window

// =====================================================
// Map Widget Code
// =====================================================

const (
	minZoom         = 7
	maxZoom         = 18
	mapTileSize     = 256
	tileResultBuf   = 64
	yourUserAgent   = "MyFyneMapApp/0.4 (contact@example.com)"
	markerRadius    = 5
	markerHitRadius = 10.0
	fetchTimeout    = 15 * time.Second
	sendTimeout     = 5 * time.Second
)

var (
	cartoSubdomains = []string{"a", "b", "c", "d"}
	mapMarkerColor  = color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	httpClient      = &http.Client{Timeout: fetchTimeout}
)

type TileCoord struct {
	Z, X, Y int
}

type TileResult struct {
	Coord TileCoord
	Image image.Image
	Error error
}

type MapMarker struct {
	ID   string
	Lat  float64
	Lon  float64
	Name string
}

type TileMapWidget struct {
	widget.BaseWidget
	mu sync.RWMutex

	zoom           int
	centerLat      float64
	centerLon      float64
	width          float32
	height         float32
	imageDataCache map[TileCoord]image.Image
	tileFetching   map[TileCoord]bool
	resultChan     chan TileResult
	stopChan       chan struct{}
	markers        []*MapMarker
	parentWindow   fyne.Window
}

func NewTileMapWidget(startZoom int, startLat, startLon float64, parentWin fyne.Window) *TileMapWidget {
	m := &TileMapWidget{
		zoom:           startZoom,
		centerLat:      startLat,
		centerLon:      startLon,
		imageDataCache: make(map[TileCoord]image.Image),
		tileFetching:   make(map[TileCoord]bool),
		resultChan:     make(chan TileResult, tileResultBuf),
		stopChan:       make(chan struct{}),
		markers:        make([]*MapMarker, 0),
		parentWindow:   parentWin,
	}
	m.ExtendBaseWidget(m)
	go m.processTileResultsLoop()
	return m
}

func (m *TileMapWidget) processTileResultsLoop() {
	for {
		select {
		case result, ok := <-m.resultChan:
			if !ok {
				return
			}
			m.handleTileResult(result)
			m.Refresh()
		case <-m.stopChan:
			return
		}
	}
}

func (m *TileMapWidget) handleTileResult(result TileResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tileFetching, result.Coord)

	if result.Error == nil && result.Image != nil {
		if result.Image.Bounds().Dx() > 0 && result.Image.Bounds().Dy() > 0 {
			m.imageDataCache[result.Coord] = result.Image
		} else {
			log.Printf("Warning: Received invalid image for tile %v (zero dimensions)", result.Coord)
		}
	} else if result.Error != nil {
		if result.Error.Error() != "tile not found (404)" {
			log.Printf("Error fetching tile %v: %v", result.Coord, result.Error)
		}
	} else {
		log.Printf("Warning: Received nil image and nil error for tile %v", result.Coord)
	}
}

func (m *TileMapWidget) AddMarkers(newMarkers ...*MapMarker) {
	if len(newMarkers) == 0 {
		return
	}
	validMarkers := make([]*MapMarker, 0, len(newMarkers))
	for _, marker := range newMarkers {
		if marker != nil {
			validMarkers = append(validMarkers, marker)
		}
	}
	if len(validMarkers) == 0 {
		return
	}
	m.mu.Lock()
	m.markers = append(m.markers, validMarkers...)
	m.mu.Unlock()
	m.Refresh()
}

func (m *TileMapWidget) latLonToScreenXY(markerLat, markerLon float64) (float32, float32) {
	m.mu.RLock()
	zoom := m.zoom
	centerLat := m.centerLat
	centerLon := m.centerLon
	w := m.width
	h := m.height
	m.mu.RUnlock()

	if w <= 0 || h <= 0 {
		return -1, -1
	}
	n := math.Pow(2.0, float64(zoom))
	markerPxX := ((markerLon + 180.0) / 360.0) * n * mapTileSize
	markerPxY := (1.0 - math.Log(math.Tan(markerLat*math.Pi/180.0)+1.0/math.Cos(markerLat*math.Pi/180.0))/math.Pi) / 2.0 * n * mapTileSize
	centerPxX := ((centerLon + 180.0) / 360.0) * n * mapTileSize
	centerPxY := (1.0 - math.Log(math.Tan(centerLat*math.Pi/180.0)+1.0/math.Cos(centerLat*math.Pi/180.0))/math.Pi) / 2.0 * n * mapTileSize
	offsetX := markerPxX - centerPxX
	offsetY := markerPxY - centerPxY
	screenX := (w / 2.0) + float32(offsetX)
	screenY := (h / 2.0) + float32(offsetY)
	return screenX, screenY
}

func (m *TileMapWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &tileMapRenderer{
		mapWidget:     m,
		canvasTiles:   make(map[TileCoord]*canvas.Image),
		canvasMarkers: make(map[*MapMarker]fyne.CanvasObject),
	}
	r.Refresh()
	return r
}

func (m *TileMapWidget) Destroy() {
	close(m.stopChan)
}

func (m *TileMapWidget) Dragged(e *fyne.DragEvent) {
	m.mu.RLock()
	currentZoom := m.zoom
	currentLat := m.centerLat
	currentLon := m.centerLon
	m.mu.RUnlock()
	centerX, centerY := latLonToTileXY(currentLat, currentLon, currentZoom)
	tileDragX := float64(e.Dragged.DX) / mapTileSize
	tileDragY := float64(e.Dragged.DY) / mapTileSize
	newCenterX := centerX - tileDragX
	newCenterY := centerY - tileDragY
	newLat, newLon := tileXYToLatLon(newCenterX, newCenterY, currentZoom)
	m.mu.Lock()
	m.centerLat = newLat
	m.centerLon = newLon
	m.mu.Unlock()
	m.clampView()
	m.Refresh()
}

func (m *TileMapWidget) DragEnd() {}

func tileXYToLatLon(xtile, ytile float64, zoom int) (lat, lon float64) {
	n := math.Pow(2.0, float64(zoom))
	lon = xtile/n*360.0 - 180.0
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*ytile/n)))
	lat = latRad * 180.0 / math.Pi
	return lat, lon
}

func (m *TileMapWidget) Scrolled(e *fyne.ScrollEvent) {
	dy := e.Scrolled.DY
	if dy == 0 {
		return
	}
	m.mu.Lock()
	zoomChanged := false
	if dy < 0 {
		if m.zoom < maxZoom {
			m.zoom++
			zoomChanged = true
		}
	} else {
		if m.zoom > minZoom {
			m.zoom--
			zoomChanged = true
		}
	}
	m.mu.Unlock()
	if zoomChanged {
		m.Refresh()
	}
}

func (m *TileMapWidget) Tapped(e *fyne.PointEvent) {
	m.mu.RLock()
	markersToCheck := make([]*MapMarker, len(m.markers))
	copy(markersToCheck, m.markers)
	m.mu.RUnlock()
	for _, marker := range markersToCheck {
		markerX, markerY := m.latLonToScreenXY(marker.Lat, marker.Lon)
		dx := e.Position.X - markerX
		dy := e.Position.Y - markerY
		distSq := dx*dx + dy*dy
		if distSq <= (markerHitRadius * markerHitRadius) {
			log.Printf("Tapped Marker: %s (%.4f, %.4f)", marker.Name, marker.Lat, marker.Lon)
			if m.parentWindow != nil {
				info := fmt.Sprintf("Marker: %s\nLat: %.6f\nLon: %.6f", marker.Name, marker.Lat, marker.Lon)
				dialog.ShowInformation("Marker Info", info, m.parentWindow)
			} else {
				log.Println("Warning: Cannot show marker dialog, parent window reference is nil")
			}
			return
		}
	}
}

func (m *TileMapWidget) clampView() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.centerLat = math.Max(-85.0511, math.Min(85.0511, m.centerLat))
	m.centerLon = math.Mod(m.centerLon+180.0, 360.0) - 180.0
	if m.centerLon == -180 {
		m.centerLon = 180
	}
}

type tileMapRenderer struct {
	mapWidget     *TileMapWidget
	objects       []fyne.CanvasObject
	canvasTiles   map[TileCoord]*canvas.Image
	canvasMarkers map[*MapMarker]fyne.CanvasObject
}

func (r *tileMapRenderer) Layout(size fyne.Size) {
	r.mapWidget.mu.Lock()
	r.mapWidget.width = size.Width
	r.mapWidget.height = size.Height
	r.mapWidget.mu.Unlock()
}

func (r *tileMapRenderer) MinSize() fyne.Size {
	return fyne.NewSize(mapTileSize, mapTileSize)
}

func (r *tileMapRenderer) Refresh() {
	r.mapWidget.mu.RLock()
	zoom := r.mapWidget.zoom
	centerLat := r.mapWidget.centerLat
	centerLon := r.mapWidget.centerLon
	width := r.mapWidget.width
	height := r.mapWidget.height
	currentMarkers := make([]*MapMarker, len(r.mapWidget.markers))
	copy(currentMarkers, r.mapWidget.markers)
	r.mapWidget.mu.RUnlock()

	if width <= 0 || height <= 0 {
		return
	}

	visibleTiles := r.calculateRequiredTiles(zoom, centerLat, centerLon, width, height)
	neededCoords := make([]TileCoord, 0, len(visibleTiles))
	activeCanvasTiles := make(map[TileCoord]bool)
	currentTileObjects := make([]fyne.CanvasObject, 0, len(visibleTiles))

	r.mapWidget.mu.Lock()
	for _, coord := range visibleTiles {
		imgData, dataFound := r.mapWidget.imageDataCache[coord]
		if dataFound {
			canvasImg, canvasFound := r.canvasTiles[coord]
			if !canvasFound {
				canvasImg = canvas.NewImageFromImage(imgData)
				if canvasImg == nil {
					log.Printf("Error: Failed to create canvas image from cached data for tile %v", coord)
					delete(r.mapWidget.imageDataCache, coord)
					continue
				}
				canvasImg.ScaleMode = canvas.ImageScaleFastest
				canvasImg.FillMode = canvas.ImageFillOriginal
				canvasImg.Resize(fyne.NewSize(mapTileSize, mapTileSize))
				r.canvasTiles[coord] = canvasImg
			}
			posX, posY := r.calculateTilePosition(coord, zoom, centerLat, centerLon, width, height)
			canvasImg.Move(fyne.NewPos(posX, posY))
			canvasImg.Show()
			currentTileObjects = append(currentTileObjects, canvasImg)
			activeCanvasTiles[coord] = true
		} else {
			if !r.mapWidget.tileFetching[coord] {
				neededCoords = append(neededCoords, coord)
				r.mapWidget.tileFetching[coord] = true
			}
		}
	}
	r.mapWidget.mu.Unlock()

	for coord, img := range r.canvasTiles {
		if !activeCanvasTiles[coord] {
			img.Hide()
			delete(r.canvasTiles, coord)
		}
	}

	currentMarkerObjects := make([]fyne.CanvasObject, 0, len(currentMarkers))
	activeCanvasMarkers := make(map[*MapMarker]bool)

	for _, marker := range currentMarkers {
		screenX, screenY := r.mapWidget.latLonToScreenXY(marker.Lat, marker.Lon)
		canvasObj, exists := r.canvasMarkers[marker]
		var circle *canvas.Circle
		if !exists {
			circle = canvas.NewCircle(mapMarkerColor)
			circle.Resize(fyne.NewSize(markerRadius*2, markerRadius*2))
			r.canvasMarkers[marker] = circle
			canvasObj = circle
		} else {
			circle = canvasObj.(*canvas.Circle)
		}
		circle.Move(fyne.NewPos(screenX-markerRadius, screenY-markerRadius))
		circle.Show()
		currentMarkerObjects = append(currentMarkerObjects, canvasObj)
		activeCanvasMarkers[marker] = true
	}

	for marker, obj := range r.canvasMarkers {
		if !activeCanvasMarkers[marker] {
			obj.Hide()
			delete(r.canvasMarkers, marker)
		}
	}

	r.objects = append(currentTileObjects, currentMarkerObjects...)
	for _, coord := range neededCoords {
		go r.fetchTileDataAsync(coord)
	}
}

func (r *tileMapRenderer) calculateRequiredTiles(zoom int, lat, lon float64, w, h float32) []TileCoord {
	centerX, centerY := latLonToTileXY(lat, lon, zoom)
	tilesX := int(math.Ceil(float64(w)/mapTileSize)) + 2
	tilesY := int(math.Ceil(float64(h)/mapTileSize)) + 2
	startX := int(math.Floor(centerX - float64(tilesX)/2.0))
	startY := int(math.Floor(centerY - float64(tilesY)/2.0))
	tiles := make([]TileCoord, 0, tilesX*tilesY)
	maxTile := int(math.Pow(2, float64(zoom))) - 1
	for x := startX; x < startX+tilesX; x++ {
		for y := startY; y < startY+tilesY; y++ {
			if y < 0 || y > maxTile {
				continue
			}
			wrappedX := x
			if maxTile >= 0 {
				nWrap := maxTile + 1
				wrappedX = (x%nWrap + nWrap) % nWrap
			} else {
				wrappedX = 0
			}
			tiles = append(tiles, TileCoord{Z: zoom, X: wrappedX, Y: y})
		}
	}
	return tiles
}

func (r *tileMapRenderer) calculateTilePosition(coord TileCoord, zoom int, centerLat, centerLon float64, w, h float32) (float32, float32) {
	n := math.Pow(2.0, float64(zoom))
	centerPxX := ((centerLon + 180.0) / 360.0) * n * mapTileSize
	centerPxY := (1.0 - math.Log(math.Tan(centerLat*math.Pi/180.0)+1.0/math.Cos(centerLat*math.Pi/180.0))/math.Pi) / 2.0 * n * mapTileSize
	tilePxX := float64(coord.X) * mapTileSize
	tilePxY := float64(coord.Y) * mapTileSize
	offsetX := tilePxX - centerPxX
	offsetY := tilePxY - centerPxY
	screenX := (w / 2.0) + float32(offsetX)
	screenY := (h / 2.0) + float32(offsetY)
	return screenX, screenY
}

func (r *tileMapRenderer) fetchTileDataAsync(coord TileCoord) {
	result := TileResult{Coord: coord}
	fetchSuccessful := false
	defer func() {
		if !fetchSuccessful {
			r.clearFetchingStatus(coord)
		}
		if rec := recover(); rec != nil {
			log.Printf("Panic recovered in fetchTileDataAsync for %v: %v", coord, rec)
			result.Error = fmt.Errorf("panic during fetch: %v", rec)
			r.sendResultNonBlocking(result)
		}
	}()

	subdomainIndex := (coord.X + coord.Y) % len(cartoSubdomains)
	subdomain := cartoSubdomains[subdomainIndex]
	url := fmt.Sprintf("https://%s.basemaps.cartocdn.com/rastertiles/voyager/%d/%d/%d.png", subdomain, coord.Z, coord.X, coord.Y)

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Error = fmt.Errorf("creating request failed: %w", err)
		r.sendResult(result)
		return
	}
	req.Header.Set("User-Agent", yourUserAgent)

	resp, err := httpClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("http timeout")
		} else if ctx.Err() == context.Canceled {
			result.Error = fmt.Errorf("http cancelled")
		} else {
			result.Error = fmt.Errorf("http request failed: %w", err)
		}
		r.sendResult(result)
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		result.Error = fmt.Errorf("tile not found (404)")
		r.sendResult(result)
		return
	}
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Errorf("http status %s", resp.Status)
		r.sendResult(result)
		return
	}

	imgData, err := png.Decode(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("decoding png failed: %w", err)
		r.sendResult(result)
		return
	}
	if imgData == nil || imgData.Bounds().Dx() <= 0 || imgData.Bounds().Dy() <= 0 {
		result.Error = fmt.Errorf("decoded image invalid")
		r.sendResult(result)
		return
	}

	result.Image = imgData
	fetchSuccessful = true
	r.sendResult(result)
}

func (r *tileMapRenderer) sendResult(result TileResult) {
	select {
	case r.mapWidget.resultChan <- result:
	case <-r.mapWidget.stopChan:
		log.Printf("Sending cancelled for tile %v result (widget stopped)", result.Coord)
		r.clearFetchingStatus(result.Coord)
	case <-time.After(sendTimeout):
		log.Printf("Timeout sending result for tile %v. Discarding.", result.Coord)
		r.clearFetchingStatus(result.Coord)
	}
}

func (r *tileMapRenderer) sendResultNonBlocking(result TileResult) {
	select {
	case r.mapWidget.resultChan <- result:
	case <-r.mapWidget.stopChan:
	default:
		log.Printf("Failed non-blocking send for tile %v", result.Coord)
	}
}

func (r *tileMapRenderer) clearFetchingStatus(coord TileCoord) {
	r.mapWidget.mu.Lock()
	delete(r.mapWidget.tileFetching, coord)
	r.mapWidget.mu.Unlock()
}

func (r *tileMapRenderer) Objects() []fyne.CanvasObject {
	r.mapWidget.mu.RLock()
	objs := make([]fyne.CanvasObject, len(r.objects))
	copy(objs, r.objects)
	r.mapWidget.mu.RUnlock()
	return objs
}

func (r *tileMapRenderer) Destroy() {}

func latLonToTileXY(lat, lon float64, zoom int) (float64, float64) {
	latRad := lat * math.Pi / 180.0
	n := math.Pow(2.0, float64(zoom))
	xtile := (lon + 180.0) / 360.0 * n
	ytile := (1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n
	return xtile, ytile
}

// =====================================================
// Chart Widget Code
// =====================================================
type connectionStatsChart struct {
	widget.BaseWidget
	title     string
	data      []float64
	maxY      float64
	yLabelMax string
	yLabelMin string
}

func newConnectionStatsChart(title string, data []float64, maxY float64, yLabelMax, yLabelMin string) *connectionStatsChart {
	c := &connectionStatsChart{
		title:     title,
		data:      data,
		maxY:      maxY,
		yLabelMax: yLabelMax,
		yLabelMin: yLabelMin,
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *connectionStatsChart) CreateRenderer() fyne.WidgetRenderer {
	dataColor := color.NRGBA{R: 0xff, G: 0xd7, B: 0x00, A: 0xff}
	titleText := canvas.NewText(c.title, theme.ForegroundColor())
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = theme.TextSize() * 1.4
	labelColor := theme.ForegroundColor()
	lineColor := theme.DisabledColor()
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
	r.yLabelMax.TextSize = theme.TextSize() * 1.1
	r.yLabelMin.TextSize = theme.TextSize() * 1.1
	r.updateDataLines()
	r.Refresh()
	return r
}

type connectionStatsChartRenderer struct {
	chart     *connectionStatsChart
	titleText *canvas.Text
	yLabelMax *canvas.Text
	yLabelMin *canvas.Text
	topLine   *canvas.Line
	midLine   *canvas.Line
	botLine   *canvas.Line
	dataLines []fyne.CanvasObject
	dataColor color.Color
}

func (r *connectionStatsChartRenderer) Layout(size fyne.Size) {
	padding := theme.Padding()
	titleHeight := float32(0)
	if r.chart.title != "" {
		titleHeight = r.titleText.MinSize().Height + padding
	}
	labelWidth := float32(math.Max(float64(r.yLabelMax.MinSize().Width), float64(r.yLabelMin.MinSize().Width))) + padding*1.5
	if r.chart.title != "" {
		r.titleText.Move(fyne.NewPos(padding, padding/2))
		r.titleText.Show()
	} else {
		r.titleText.Hide()
	}
	chartAreaX := labelWidth
	chartAreaWidth := size.Width - labelWidth - padding
	chartAreaY := titleHeight + r.yLabelMax.MinSize().Height/2
	chartAreaHeight := size.Height - chartAreaY - r.yLabelMin.MinSize().Height/2 - padding
	if chartAreaWidth < 0 {
		chartAreaWidth = 0
	}
	if chartAreaHeight < 0 {
		chartAreaHeight = 0
	}
	r.yLabelMax.Move(fyne.NewPos(padding, chartAreaY-r.yLabelMax.MinSize().Height/2))
	r.yLabelMin.Move(fyne.NewPos(padding, size.Height-padding-r.yLabelMin.MinSize().Height/2))
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
			if dataHeight < 0 {
				dataHeight = 0
			}
			yPosData := botY - dataHeight
			castLine.Position1 = fyne.NewPos(xPos, botY)
			castLine.Position2 = fyne.NewPos(xPos, yPosData)
			castLine.StrokeWidth = float32(math.Max(1.0, float64(stepX*0.95)))
		}
	}
}

func (r *connectionStatsChartRenderer) MinSize() fyne.Size {
	titleMinHeight, titleMinWidth := float32(0), float32(0)
	if r.chart.title != "" {
		titleMin := r.titleText.MinSize()
		titleMinHeight = titleMin.Height
		titleMinWidth = titleMin.Width
	}
	labelWidth := float32(math.Max(float64(r.yLabelMax.MinSize().Width), float64(r.yLabelMin.MinSize().Width)))
	minChartAreaHeight := theme.TextSize() * 15
	minChartAreaWidth := float32(50)
	minHeight := titleMinHeight + r.yLabelMax.MinSize().Height + r.yLabelMin.MinSize().Height + minChartAreaHeight + theme.Padding()*4
	minWidth := float32(math.Max(float64(titleMinWidth), float64(labelWidth+minChartAreaWidth))) + theme.Padding()*3
	return fyne.NewSize(minWidth, minHeight)
}

func (r *connectionStatsChartRenderer) Refresh() {
	r.titleText.Color = theme.ForegroundColor()
	r.yLabelMax.Color = theme.ForegroundColor()
	r.yLabelMin.Color = theme.ForegroundColor()
	r.topLine.StrokeColor = theme.DisabledColor()
	r.midLine.StrokeColor = theme.DisabledColor()
	r.botLine.StrokeColor = theme.DisabledColor()
	r.titleText.Text = r.chart.title
	r.yLabelMax.Text = r.chart.yLabelMax
	r.yLabelMin.Text = r.chart.yLabelMin
	r.titleText.Refresh()
	r.yLabelMax.Refresh()
	r.yLabelMin.Refresh()
	r.updateDataLines()
	r.topLine.Refresh()
	r.midLine.Refresh()
	r.botLine.Refresh()
	for _, line := range r.dataLines {
		line.Refresh()
	}
	r.Layout(r.chart.Size())
}

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
	for _, obj := range r.dataLines {
		if line, ok := obj.(*canvas.Line); ok {
			line.StrokeColor = r.dataColor
		}
	}
}

func (r *connectionStatsChartRenderer) Objects() []fyne.CanvasObject {
	objects := r.dataLines
	objects = append(objects, r.topLine, r.midLine, r.botLine)
	if r.chart.title != "" {
		objects = append(objects, r.titleText)
	}
	objects = append(objects, r.yLabelMax, r.yLabelMin)
	return objects
}

func (r *connectionStatsChartRenderer) Destroy() {}

// =====================================================
// Application Screens
// =====================================================

func main() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())
	mainWindow = a.NewWindow("Fyne App with Map")
	loginContent := createLoginScreen()
	mainWindow.SetContent(loginContent)
	mainWindow.Resize(fyne.NewSize(1000, 700))
	mainWindow.CenterOnScreen()
	mainWindow.ShowAndRun()
	log.Println("Application exiting.")
}

func createLoginScreen() fyne.CanvasObject {
	userNameLabel := widget.NewLabel("John Doe")
	topContent := container.NewHBox(layout.NewSpacer(), userNameLabel)
	titleLabel := widget.NewLabelWithStyle("Connect Bridge", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	signInButton := widget.NewButton("[Signin]", func() {
		fmt.Println("Sign in clicked!")
		loggedInContent := createLoggedInScreen()
		mainWindow.SetContent(loggedInContent)
	})
	centerBox := container.NewVBox(layout.NewSpacer(), titleLabel, signInButton, layout.NewSpacer())
	centerContent := container.NewCenter(centerBox)
	loginLayout := container.NewBorder(container.NewPadded(topContent), nil, nil, nil, centerContent)
	return loginLayout
}

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
		lat    float64
		lon    float64
	}{
		{"Ho Chi Minh Office", "ap-southeast-3", 10.7769, 106.7009},
		{"Thailand Office", "ap-southeast-2", 13.7563, 100.5018},
		{"Germany Office", "europe-3", 50.1109, 8.6821},
	}

	gatewayMarkers := make([]*MapMarker, 0, len(gateways))
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

		gatewayMarkers = append(gatewayMarkers, &MapMarker{
			ID:   gw.region,
			Lat:  gw.lat,
			Lon:  gw.lon,
			Name: gwName,
		})
	}

	leftSideContent := container.NewVBox(
		quickConnectButton,
		widget.NewSeparator(),
		tabsHeader,
		widget.NewSeparator(),
		gatewayList,
		layout.NewSpacer(),
	)

	mapStartZoom := 12
	mapStartLat := gateways[0].lat
	mapStartLon := gateways[0].lon

	mapWidget := NewTileMapWidget(mapStartZoom, mapStartLat, mapStartLon, mainWindow)
	mapWidget.AddMarkers(gatewayMarkers...)

	rightSideContent := container.NewMax(mapWidget)

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

func createConnectedScreen(gatewayName, gatewayRegion, deviceName string) fyne.CanvasObject {

	userNameLabel := widget.NewLabel("Vinh Nguyen")
	logoutButton := widget.NewButton("[Logout]", func() {
		fmt.Println("Logout clicked")
		loginContent := createLoginScreen()
		mainWindow.SetContent(loginContent)
	})
	topBar := container.NewHBox(layout.NewSpacer(), userNameLabel, logoutButton)

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
	connectionInfoSection := container.NewVBox(statusLabel, connectionDetailsHBox, disconnectButton)

	chartData := []float64{0.1, 0.1, 0.2, 0.1, 0.2, 0.3, 0.2, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.2, 0.3, 0.5, 0.8, 1.2, 1.8, 2.5, 3.0, 2.2, 1.5, 0.8, 0.5, 0.3, 0.2, 0.2, 0.2, 0.3, 0.4, 0.8, 1.5, 2.5, 4.0, 6.0, 8.5, 10.0, 11.0, 10.8, 9.5, 8.0, 6.5, 5.5, 4.8, 4.0, 3.5, 3.0, 2.6, 2.2, 1.8, 1.5, 1.2, 1.0, 0.8, 0.6, 0.5, 0.4, 0.3, 0.2, 0.2, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	maxYValue := 11.2
	chartWidget := newConnectionStatsChart("Connction Stats", chartData, maxYValue, "11.2KB/s", "0B/s")
	chartContainer := container.NewMax(chartWidget)

	bytesInIcon := widget.NewIcon(theme.MoveDownIcon())
	bytesInLabel := widget.NewLabel("BYTES IN\n0 KB/S")
	bytesInSection := container.NewHBox(bytesInIcon, bytesInLabel)
	bytesOutIcon := widget.NewIcon(theme.MoveUpIcon())
	bytesOutLabel := widget.NewLabel("BYTES OUT\n0 KB/S")
	bytesOutSection := container.NewHBox(bytesOutIcon, bytesOutLabel)
	bytesHBox := container.NewHBox(bytesInSection, layout.NewSpacer(), bytesOutSection)

	centerContent := container.NewVBox(
		connectionInfoSection,
		widget.NewSeparator(),
		chartContainer,
		widget.NewSeparator(),
		bytesHBox,
		layout.NewSpacer(),
	)

	connectedLayout := container.NewBorder(
		container.NewPadded(topBar),
		nil, nil, nil,
		container.NewPadded(centerContent),
	)
	return connectedLayout
}
