package main

import (
	"fmt"
	"image/color" // Needed for chart placeholder color

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas" // Needed for chart placeholder
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme" // Needed for icons
	"fyne.io/fyne/v2/widget"
)

// Global reference to the main window
var mainWindow fyne.Window

// --- Main Application ---
func main() {
	a := app.New()
	// To match the dark screenshot theme:
	a.Settings().SetTheme(theme.DarkTheme())

	mainWindow = a.NewWindow("Fyne App")

	// Start with the login screen
	loginContent := createLoginScreen()
	mainWindow.SetContent(loginContent)

	mainWindow.Resize(fyne.NewSize(800, 600)) // Increased size slightly
	mainWindow.CenterOnScreen()
	mainWindow.Show()

	a.Run()
}

// --- Login Screen ---
func createLoginScreen() fyne.CanvasObject {
	// (Same as before, just ensure the button navigates correctly)
	userNameLabel := widget.NewLabel("John Doe") // Or use Vinh Nguyen if preferred start state
	topContent := container.NewHBox(layout.NewSpacer(), userNameLabel)

	titleLabel := widget.NewLabelWithStyle(
		"Connect Bridge",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	signInButton := widget.NewButton("[Signin]", func() {
		fmt.Println("Sign in clicked!")
		loggedInContent := createLoggedInScreen()
		mainWindow.SetContent(loggedInContent) // Navigate to logged in screen
	})

	centerBox := container.NewVBox(
		layout.NewSpacer(), // Pushes content down slightly
		titleLabel,
		signInButton,
		layout.NewSpacer(), // Pushes content up slightly
	)
	centerContent := container.NewCenter(centerBox)

	loginLayout := container.NewBorder(
		container.NewPadded(topContent), // Add padding
		nil,
		nil,
		nil,
		centerContent,
	)

	return loginLayout
}

// --- Logged In Screen (Gateway List) ---
func createLoggedInScreen() fyne.CanvasObject {
	// --- Top Right User Info ---
	userName := widget.NewLabel("Vinh Nguyen")
	userEmail := widget.NewLabel("vinh@gmail.com")
	userInfo := container.NewVBox(
		userName,
		userEmail,
	)
	topBar := container.NewHBox(layout.NewSpacer(), userInfo)

	// --- Center Content Area (Split Left/Right) ---

	// -- Left Side --
	quickConnectButton := widget.NewButton("[Quick Connect]", func() {
		fmt.Println("Quick Connect clicked")
		// Navigate to Connected Screen with generic info
		connectedContent := createConnectedScreen("Quick Connect Gateway", "auto-region", "your-device (IP unknown)")
		mainWindow.SetContent(connectedContent)
	})

	gatewaysLabel := widget.NewLabelWithStyle("Gateways", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	recentConnectionsLabel := widget.NewLabel("Recent connections")
	// Using a simple HBox for tabs header, not functional tabs
	tabsHeader := container.NewHBox(gatewaysLabel, layout.NewSpacer(), recentConnectionsLabel)

	// Gateway List Items (Example Data)
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
		// Capture loop variables for the closure!
		gwName := gw.name
		gwRegion := gw.region

		nameLabel := widget.NewLabel(gwName)
		regionLabel := widget.NewLabel(gwRegion)
		infoVBox := container.NewVBox(nameLabel, regionLabel)

		connectButton := widget.NewButton("[Connect]", func() {
			fmt.Printf("Connect clicked for: %s\n", gwName)
			// Navigate to Connected Screen with specific gateway info
			// You would typically get the device name/IP from the connection process
			deviceName := "john-laptop (100.100.24.3)" // Example device info
			connectedContent := createConnectedScreen(gwName, gwRegion, deviceName)
			mainWindow.SetContent(connectedContent)
		})

		row := container.NewHBox(infoVBox, layout.NewSpacer(), connectButton)
		gatewayList.Add(row)
		gatewayList.Add(widget.NewSeparator()) // Separator between entries
	}

	leftSideContent := container.NewVBox(
		quickConnectButton,
		widget.NewSeparator(),
		tabsHeader,
		widget.NewSeparator(),
		gatewayList,
		layout.NewSpacer(), // Push content upwards
	)

	// -- Right Side --
	mapPlaceholder := widget.NewLabel("the map for available gateways")
	rightSideContent := container.NewCenter(mapPlaceholder)

	// Use HSplit
	centerSplit := container.NewHSplit(
		container.NewPadded(leftSideContent),
		container.NewPadded(rightSideContent),
	)
	centerSplit.Offset = 0.4

	// --- Assemble Logged In Layout ---
	loggedInLayout := container.NewBorder(
		container.NewPadded(topBar),
		nil,
		nil,
		nil,
		centerSplit,
	)

	return loggedInLayout
}

// --- Connected Screen (Connection Details & Stats) ---
func createConnectedScreen(gatewayName, gatewayRegion, deviceName string) fyne.CanvasObject {

	// --- Top Right User Info & Logout ---
	userNameLabel := widget.NewLabel("Vinh Nguyen")
	logoutButton := widget.NewButton("[Logout]", func() {
		fmt.Println("Logout clicked")
		// Navigate back to the initial login screen
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

	// Arrange gateway details and device info horizontally
	connectionDetailsHBox := container.NewHBox(gatewayDetailsVBox, layout.NewSpacer(), deviceLabel)

	disconnectButton := widget.NewButton("[Disconnect]", func() {
		fmt.Println("Disconnect clicked")
		// Navigate back to the gateway list screen
		loggedInContent := createLoggedInScreen()
		mainWindow.SetContent(loggedInContent)
	})

	connectionInfoSection := container.NewVBox(
		statusLabel,
		connectionDetailsHBox,
		disconnectButton,
	)

	// Connection Stats Section
	statsTitle := widget.NewLabelWithStyle("Connection Stats", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	currentSpeedLabel := widget.NewLabel("11.2KB/s") // Placeholder value

	// Chart Placeholder - Using a simple colored rectangle
	chartPlaceholder := canvas.NewRectangle(color.NRGBA{R: 255, G: 165, B: 0, A: 150}) // Orange-ish, semi-transparent
	chartPlaceholder.SetMinSize(fyne.NewSize(200, 100))                                // Give it a minimum size
	chartContainer := container.NewMax(chartPlaceholder)                               // Max container helps control size within layout

	// Bytes In/Out Section
	bytesInIcon := widget.NewIcon(theme.ContentAddIcon()) // Replace with a valid icon
	bytesInLabel := widget.NewLabel("BYTES IN\n0 KB/S")   // Multi-line
	bytesInSection := container.NewHBox(bytesInIcon, bytesInLabel)

	bytesOutIcon := widget.NewIcon(theme.ContentRemoveIcon())
	bytesOutLabel := widget.NewLabel("BYTES OUT\n0 KB/S") // Multi-line
	bytesOutSection := container.NewHBox(bytesOutIcon, bytesOutLabel)

	bytesHBox := container.NewHBox(bytesInSection, layout.NewSpacer(), bytesOutSection)

	// Assemble Center VBox
	centerContent := container.NewVBox(
		connectionInfoSection,
		widget.NewSeparator(),
		statsTitle,
		currentSpeedLabel,
		chartContainer, // Add the chart placeholder container
		widget.NewSeparator(),
		bytesHBox,
		layout.NewSpacer(), // Push content up
	)

	// --- Assemble Connected Layout ---
	connectedLayout := container.NewBorder(
		container.NewPadded(topBar),
		nil,
		nil,
		nil,
		container.NewPadded(centerContent), // Pad the central content
	)

	return connectedLayout
}
