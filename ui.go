//go:build gui
// +build gui

package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fyne.io/fyne/v2/driver/desktop"
)

// Custom Windows-style theme
type windowsTheme struct {
	fyne.Theme
	darkMode bool
}

func newWindowsTheme(darkMode bool) fyne.Theme {
	return &windowsTheme{theme.DefaultTheme(), darkMode}
}

func (t *windowsTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.darkMode {
		return t.darkColor(name)
	}
	return t.lightColor(name)
}

func (t *windowsTheme) lightColor(name fyne.ThemeColorName) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff}
	case theme.ColorNameButton:
		return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameDisabledButton:
		return color.RGBA{R: 0xe1, G: 0xe1, B: 0xe1, A: 0xff}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 0x00, G: 0x78, B: 0xd4, A: 0xff}
	case theme.ColorNameFocus:
		return color.RGBA{R: 0x00, G: 0x78, B: 0xd4, A: 0xff}
	case theme.ColorNameSelection:
		return color.RGBA{R: 0x00, G: 0x78, B: 0xd4, A: 0xff}
	case theme.ColorNameHeaderBackground:
		return color.RGBA{R: 0xfa, G: 0xfa, B: 0xfa, A: 0xff}
	case theme.ColorNameSeparator:
		return color.RGBA{R: 0xd1, G: 0xd1, B: 0xd1, A: 0xff}
	case theme.ColorNameSuccess:
		return color.RGBA{R: 0x10, G: 0x7c, B: 0x10, A: 0xff}
	case theme.ColorNameError:
		return color.RGBA{R: 0xc4, G: 0x2b, B: 0x1c, A: 0xff}
	case theme.ColorNameForeground:
		return color.RGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 0x8c, G: 0x8c, B: 0x8c, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 0x6c, G: 0x6c, B: 0x6c, A: 0xff}
	case theme.ColorNameHover:
		return color.RGBA{R: 0xe5, G: 0xf3, B: 0xff, A: 0xff}
	case theme.ColorNamePressed:
		return color.RGBA{R: 0xcc, G: 0xe4, B: 0xf7, A: 0xff}
	case theme.ColorNameScrollBar:
		return color.RGBA{R: 0xc1, G: 0xc1, B: 0xc1, A: 0xff}
	case theme.ColorNameShadow:
		return color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x20}
	case theme.ColorNameInputBorder:
		return color.RGBA{R: 0x8c, G: 0x8c, B: 0x8c, A: 0xff}
	case theme.ColorNameMenuBackground:
		return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameOverlayBackground:
		return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	return t.Theme.Color(name, theme.VariantLight)
}

func (t *windowsTheme) darkColor(name fyne.ThemeColorName) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff}
	case theme.ColorNameButton:
		return color.RGBA{R: 0x2d, G: 0x2d, B: 0x2d, A: 0xff}
	case theme.ColorNameDisabledButton:
		return color.RGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 0x2d, G: 0x2d, B: 0x2d, A: 0xff}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 0x60, G: 0xcd, B: 0xfa, A: 0xff}
	case theme.ColorNameFocus:
		return color.RGBA{R: 0x60, G: 0xcd, B: 0xfa, A: 0xff}
	case theme.ColorNameSelection:
		return color.RGBA{R: 0x26, G: 0x82, B: 0xc4, A: 0xff}
	case theme.ColorNameHeaderBackground:
		return color.RGBA{R: 0x2d, G: 0x2d, B: 0x2d, A: 0xff}
	case theme.ColorNameSeparator:
		return color.RGBA{R: 0x3d, G: 0x3d, B: 0x3d, A: 0xff}
	case theme.ColorNameSuccess:
		return color.RGBA{R: 0x4c, G: 0xaf, B: 0x50, A: 0xff}
	case theme.ColorNameError:
		return color.RGBA{R: 0xef, G: 0x53, B: 0x50, A: 0xff}
	case theme.ColorNameForeground:
		return color.RGBA{R: 0xe3, G: 0xe3, B: 0xe3, A: 0xff}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 0x6e, G: 0x6e, B: 0x6e, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 0x8a, G: 0x8a, B: 0x8a, A: 0xff}
	case theme.ColorNameHover:
		return color.RGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff}
	case theme.ColorNamePressed:
		return color.RGBA{R: 0x4a, G: 0x4a, B: 0x4a, A: 0xff}
	case theme.ColorNameScrollBar:
		return color.RGBA{R: 0x4d, G: 0x4d, B: 0x4d, A: 0xff}
	case theme.ColorNameShadow:
		return color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x40}
	case theme.ColorNameInputBorder:
		return color.RGBA{R: 0x4d, G: 0x4d, B: 0x4d, A: 0xff}
	case theme.ColorNameMenuBackground:
		return color.RGBA{R: 0x2d, G: 0x2d, B: 0x2d, A: 0xff}
	case theme.ColorNameOverlayBackground:
		return color.RGBA{R: 0x2d, G: 0x2d, B: 0x2d, A: 0xff}
	}
	return t.Theme.Color(name, theme.VariantDark)
}

// UI holds all GUI components
type UI struct {
	app       fyne.App
	window    fyne.Window
	processor *Processor
	config    *Config

	// Header
	titleLabel *widget.Label

	// Toolbar buttons
	btnOpenFolder  *widget.Button
	btnAddImages   *widget.Button
	btnClear       *widget.Button
	btnSettings    *widget.Button
	btnSelectAll   *widget.Button
	btnDeselectAll *widget.Button

	// Action buttons
	btnStart *widget.Button
	btnStop  *widget.Button

	// List components
	imageList     *widget.List
	listData      []*Task
	listMutex     sync.RWMutex
	emptyLabel    *widget.Label
	listContainer *fyne.Container
	selectedMap   map[int]bool             // Track selection state by index
	checkboxes    map[int]*widget.Check    // Store checkbox references
	listWidgets   map[int]*listItemWidgets // Store widget references by index
	widgetsMutex  sync.RWMutex

	// Thumbnail cache with limit
	thumbnailCache map[string]image.Image
	thumbMutex     sync.RWMutex
	thumbOrder     []string // Track insertion order for LRU
	maxThumbnails  int

	// Status bar
	statusLabel    *widget.Label
	progressBar    *widget.ProgressBar
	progressLabel  *widget.Label
	totalLabel     *widget.Label
	completedLabel *widget.Label
	failedLabel    *widget.Label
	balanceLabel   *widget.Label

	// Containers
	mainContainer *fyne.Container

	// State
	isProcessing bool

	// Context menu
	contextMenu      *widget.PopUpMenu
	contextMenuIndex int // Index of the item that was right-clicked

	// Drag and drop
	dropWell *widget.Label

	// Search and sort
	searchEntry   *widget.Entry
	sortSelect    *widget.Select
	sortAscending bool
	filterText    string
	promptLabel   *widget.Label
}

// NewUI creates the main application UI
func NewUI(app fyne.App, processor *Processor, config *Config) *UI {
	// Set theme based on config
	darkMode := config.Theme == "dark"
	app.Settings().SetTheme(newWindowsTheme(darkMode))

	ui := &UI{
		app:            app,
		window:         app.NewWindow("LuminaFlow"),
		processor:      processor,
		config:         config,
		listData:       make([]*Task, 0),
		thumbnailCache: make(map[string]image.Image),
		thumbOrder:     make([]string, 0),
		maxThumbnails:  100, // Limit cache to 100 images
		selectedMap:    make(map[int]bool),
		checkboxes:     make(map[int]*widget.Check),
		listWidgets:    make(map[int]*listItemWidgets),
	}

	ui.setupUI()
	return ui
}

func (ui *UI) setupUI() {
	ui.window.Resize(fyne.NewSize(900, 650))
	ui.window.SetMaster()

	// Header with title and version
	titleText := fmt.Sprintf("%s v%s - 图片转视频工具", AppName, AppVersion)
	ui.titleLabel = widget.NewLabelWithStyle(titleText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	ui.titleLabel.Importance = widget.HighImportance

	// Toolbar buttons (top row)
	ui.btnOpenFolder = widget.NewButtonWithIcon("选择文件夹", theme.FolderOpenIcon(), ui.onBrowseFolder)
	ui.btnAddImages = widget.NewButtonWithIcon("添加图片", theme.ContentAddIcon(), ui.onAddFiles)
	ui.btnClear = widget.NewButtonWithIcon("清空列表", theme.DeleteIcon(), ui.onClearList)
	ui.btnClear.Disable()

	ui.btnSettings = widget.NewButtonWithIcon("设置", theme.SettingsIcon(), ui.onSettings)
	btnHelp := widget.NewButtonWithIcon("使用说明", theme.HelpIcon(), ui.onHelp)

	// Selection buttons
	ui.btnSelectAll = widget.NewButton("全选", ui.onSelectAll)
	ui.btnDeselectAll = widget.NewButton("全不选", ui.onDeselectAll)

	toolbarRow := container.NewHBox(
		ui.btnOpenFolder,
		ui.btnAddImages,
		ui.btnClear,
		widget.NewSeparator(),
		ui.btnSelectAll,
		ui.btnDeselectAll,
		layout.NewSpacer(),
		btnHelp,
		ui.btnSettings,
	)

	// Action buttons (second row)
	ui.btnStart = widget.NewButtonWithIcon("开始转换", theme.MediaPlayIcon(), ui.onStart)
	ui.btnStart.Importance = widget.HighImportance
	ui.btnStart.Disable()

	ui.btnStop = widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), ui.onStop)
	ui.btnStop.Hide()

	// Progress percentage label
	ui.progressLabel = widget.NewLabel("")

	actionRow := container.NewHBox(
		ui.btnStart,
		ui.btnStop,
		ui.progressLabel,
		layout.NewSpacer(),
	)

	// Search and sort row with proper layout
	ui.searchEntry = widget.NewEntry()
	ui.searchEntry.SetPlaceHolder("搜索文件名...")
	ui.searchEntry.OnChanged = func(text string) {
		ui.filterText = strings.ToLower(text)
		ui.refreshList()
	}

	ui.sortSelect = widget.NewSelect([]string{"默认顺序", "按文件名", "按状态", "按尺寸"}, func(s string) {
		ui.sortTasks()
		ui.refreshList()
	})
	ui.sortSelect.SetSelected("默认顺序")
	ui.sortAscending = true

	sortBtn := widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), func() {
		ui.sortAscending = !ui.sortAscending
		ui.sortTasks()
		ui.refreshList()
	})

	// Search row - use Border layout to make search entry expand
	sortControls := container.NewHBox(
		widget.NewLabel("排序:"),
		ui.sortSelect,
		sortBtn,
	)
	searchRow := container.NewBorder(nil, nil,
		widget.NewLabel("搜索:"),
		sortControls,
		ui.searchEntry)

	// Image list
	ui.emptyLabel = widget.NewLabel("请点击「选择文件夹」或「添加图片」来加载图片\n\n支持格式: JPG, PNG, WEBP\n\n也可以直接拖拽图片到此处\n\n⚠️ 使用前请先在「设置」中配置 DMXAPI Key")
	ui.emptyLabel.Alignment = fyne.TextAlignCenter

	ui.imageList = widget.NewList(
		ui.listItemCount,
		ui.listItemCreate,
		ui.listItemUpdate,
	)

	ui.listContainer = container.NewStack(ui.imageList, ui.emptyLabel)

	// Setup drag and drop
	ui.setupDragAndDrop()

	// Status bar
	ui.statusLabel = widget.NewLabel("就绪")
	ui.progressBar = widget.NewProgressBar()
	ui.progressBar.Hide()
	ui.totalLabel = widget.NewLabel("总计: 0")
	ui.completedLabel = widget.NewLabel("完成: 0")
	ui.failedLabel = widget.NewLabel("失败: 0")
	developerLabel := widget.NewLabel("上海觉测信息科技有限公司")

	// Prompt display
	ui.promptLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	ui.promptLabel.Wrapping = fyne.TextTruncate
	ui.updatePromptDisplay()

	// Balance display with refresh button
	ui.balanceLabel = widget.NewLabelWithStyle("令牌剩余次数: -- | 余额: --", fyne.TextAlignLeading, fyne.TextStyle{})
	btnRefreshBalance := widget.NewButtonWithIcon("刷新", theme.ViewRefreshIcon(), func() {
		go ui.queryBalance()
	})
	btnRefreshBalance.Importance = widget.LowImportance

	// Balance section
	balanceSection := container.NewHBox(
		ui.balanceLabel,
		btnRefreshBalance,
	)

	// Task statistics section
	statsSection := container.NewHBox(
		ui.totalLabel,
		widget.NewSeparator(),
		ui.completedLabel,
		widget.NewSeparator(),
		ui.failedLabel,
	)

	statusBar := container.NewBorder(
		nil, nil,
		ui.statusLabel,
		container.NewHBox(statsSection, widget.NewSeparator(), balanceSection, widget.NewSeparator(), developerLabel),
	)

	// Progress bar row with prompt
	progressRow := container.NewBorder(nil, nil, nil, nil, ui.progressBar)

	// Main layout
	headerSection := container.NewVBox(
		ui.titleLabel,
		widget.NewSeparator(),
		toolbarRow,
		actionRow,
		searchRow,
		widget.NewSeparator(),
	)

	// Footer with progress, prompt and status
	footerSection := container.NewVBox(
		widget.NewSeparator(),
		progressRow,
		ui.promptLabel,
		statusBar,
	)

	ui.mainContainer = container.NewBorder(
		headerSection,
		footerSection,
		nil,
		nil,
		ui.listContainer,
	)

	ui.window.SetContent(ui.mainContainer)

	// Start background tasks
	go ui.eventListener()
	go ui.updateLoop()
	go ui.balanceRefreshLoop()

	// Setup keyboard shortcuts
	ui.setupShortcuts()

	// Initial balance query
	go ui.queryBalance()
}

// setupShortcuts configures keyboard shortcuts
func (ui *UI) setupShortcuts() {
	// Ctrl+O: Open folder
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		ui.onBrowseFolder()
	})

	// Ctrl+A: Select all
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		ui.onSelectAll()
	})

	// Ctrl+D: Deselect all
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		ui.onDeselectAll()
	})

	// Ctrl+Enter: Start conversion
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		if ui.processor.IsRunning() {
			return
		}
		if ui.processor.GetSelectedCount() > 0 {
			ui.onStart()
		}
	})

	// Escape: Stop conversion
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEscape, Modifier: 0}, func(_ fyne.Shortcut) {
		if ui.processor.IsRunning() {
			ui.onStop()
		}
	})

	// F1: Help
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF1, Modifier: 0}, func(_ fyne.Shortcut) {
		ui.onHelp()
	})

	// F5: Refresh list
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF5, Modifier: 0}, func(_ fyne.Shortcut) {
		ui.imageList.Refresh()
		ui.updateStatusBar()
	})

	// Ctrl+Delete: Clear list
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyDelete, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		if !ui.processor.IsRunning() {
			ui.onClearList()
		}
	})

	// Ctrl+Comma: Settings
	ui.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyComma, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		ui.onSettings()
	})

	Info("Keyboard shortcuts configured")
}

// setupDragAndDrop enables drag and drop functionality
func (ui *UI) setupDragAndDrop() {
	ui.window.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}

		if ui.processor.IsRunning() {
			dialog.ShowInformation("提示", "请等待当前处理完成后再添加图片", ui.window)
			return
		}

		var images []ImageInfo
		for _, uri := range uris {
			// Check if it's a file URI
			if uri.Scheme() != "file" {
				continue
			}

			path := uri.Path()

			// Check if it's a directory
			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			if info.IsDir() {
				// Scan directory for images
				dirImages, err := ScanImages(path)
				if err != nil {
					continue
				}
				images = append(images, dirImages...)
			} else {
				// Check if it's a supported image file
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
					imgInfo, err := LoadImageInfo(path)
					if err != nil {
						Warn("Failed to get image info for %s: %v", path, err)
						continue
					}
					images = append(images, *imgInfo)
				}
			}
		}

		if len(images) == 0 {
			dialog.ShowInformation("提示", "没有找到支持的图片文件\n\n支持格式: JPG, PNG, WEBP", ui.window)
			return
		}

		ui.addImages(images)
	})
}

func (ui *UI) Show() {
	ui.window.Show()
}

func (ui *UI) Run() {
	ui.app.Run()
}

func (ui *UI) onBrowseFolder() {
	cwd, _ := os.Getwd()

	fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil || dir == nil {
			return
		}

		ui.statusLabel.SetText("正在扫描文件夹...")
		images, err := ScanImages(dir.Path())
		if err != nil {
			dialog.ShowError(err, ui.window)
			ui.statusLabel.SetText("就绪")
			return
		}

		if len(images) == 0 {
			dialog.ShowInformation("提示", "所选文件夹中没有找到支持的图片文件\n\n支持格式: JPG, PNG, WEBP", ui.window)
			ui.statusLabel.SetText("就绪")
			return
		}

		ui.showConfirmDialog(images)
	}, ui.window)

	if lister, err := storage.ListerForURI(storage.NewFileURI(cwd)); err == nil {
		fd.SetLocation(lister)
	}
	fd.Show()
}

func (ui *UI) onAddFiles() {
	cwd, _ := os.Getwd()
	ui.showImageSelectorDialog(cwd)
}

// showImageSelectorDialog displays a dialog with image thumbnails for multi-selection
func (ui *UI) showImageSelectorDialog(folderPath string) {
	ui.statusLabel.SetText("正在扫描文件夹...")

	images, err := ScanImages(folderPath)
	if err != nil {
		dialog.ShowError(err, ui.window)
		ui.statusLabel.SetText("就绪")
		return
	}

	if len(images) == 0 {
		dialog.ShowInformation("提示", "当前文件夹中没有找到支持的图片文件\n\n支持格式: JPG, PNG, WEBP", ui.window)
		ui.statusLabel.SetText("就绪")
		return
	}

	selected := make(map[int]bool)
	var checkList []*widget.Check

	for i, img := range images {
		idx := i
		check := widget.NewCheck(fmt.Sprintf("%s (%dx%d)", img.Filename, img.Width, img.Height), func(checked bool) {
			selected[idx] = checked
		})
		check.SetChecked(true)
		selected[idx] = true
		checkList = append(checkList, check)
	}

	listContainer := container.NewVBox()
	for _, check := range checkList {
		listContainer.Add(check)
	}

	scroll := container.NewVScroll(listContainer)
	scroll.SetMinSize(fyne.NewSize(400, 300))

	selectAllBtn := widget.NewButton("全选", func() {
		for i := range checkList {
			checkList[i].SetChecked(true)
			selected[i] = true
		}
	})
	deselectAllBtn := widget.NewButton("全不选", func() {
		for i := range checkList {
			checkList[i].SetChecked(false)
			selected[i] = false
		}
	})

	changeFolderBtn := widget.NewButtonWithIcon("选择其他文件夹", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
			if err != nil || dir == nil {
				return
			}
			ui.showImageSelectorDialog(dir.Path())
		}, ui.window)
		if lister, err := storage.ListerForURI(storage.NewFileURI(folderPath)); err == nil {
			fd.SetLocation(lister)
		}
		fd.Show()
	})

	buttonRow := container.NewHBox(selectAllBtn, deselectAllBtn, changeFolderBtn)
	countLabel := widget.NewLabel(fmt.Sprintf("当前目录: %s\n共 %d 个图片", folderPath, len(images)))

	content := container.NewBorder(
		container.NewVBox(countLabel, buttonRow, widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)

	d := dialog.NewCustomConfirm("选择要转换的图片", "添加", "取消", content,
		func(confirmed bool) {
			if !confirmed {
				ui.statusLabel.SetText("就绪")
				return
			}

			var selectedImages []ImageInfo
			for i, img := range images {
				if selected[i] {
					selectedImages = append(selectedImages, img)
				}
			}

			if len(selectedImages) == 0 {
				dialog.ShowInformation("提示", "请至少选择一个图片", ui.window)
				ui.statusLabel.SetText("就绪")
				return
			}

			ui.addImages(selectedImages)
			ui.statusLabel.SetText("就绪")
		}, ui.window)

	d.Resize(fyne.NewSize(500, 450))
	d.Show()
}

func (ui *UI) showConfirmDialog(images []ImageInfo) {
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("已选择 %d 个图片文件:\n\n", len(images)))

	maxShow := 8
	for i, img := range images {
		if i >= maxShow {
			summary.WriteString(fmt.Sprintf("\n... 还有 %d 个文件", len(images)-maxShow))
			break
		}
		summary.WriteString(fmt.Sprintf("  %s  (%dx%d)\n", img.Filename, img.Width, img.Height))
	}

	summary.WriteString("\n\n是否添加到转换列表?")

	dialog.ShowConfirm("确认添加图片", summary.String(),
		func(confirmed bool) {
			if confirmed {
				ui.addImages(images)
			}
			ui.statusLabel.SetText("就绪")
		}, ui.window)
}

func (ui *UI) addImages(images []ImageInfo) {
	Info("addImages: adding %d images", len(images))

	// Get current count before adding
	ui.listMutex.RLock()
	startIndex := len(ui.listData)
	ui.listMutex.RUnlock()

	ui.processor.AddImages(images)

	// Initialize selection state for new images (default to selected)
	for i := 0; i < len(images); i++ {
		ui.selectedMap[startIndex+i] = true
		ui.processor.SetTaskSelected(startIndex+i, true)
	}

	Info("addImages: calling refreshList")
	ui.refreshList()
	Info("addImages: refreshList done, enabling buttons")
	ui.btnStart.Enable()
	ui.btnClear.Enable()
	ui.updateStatusBar()

	// Preload thumbnails in background
	go ui.preloadThumbnails(images)
}

// preloadThumbnails loads thumbnails in background with LRU cache eviction
func (ui *UI) preloadThumbnails(images []ImageInfo) {
	Info("Preloading %d thumbnails...", len(images))

	for i, img := range images {
		thumb, err := LoadImageForThumbnail(img.Path)
		if err != nil {
			Warn("Failed to load thumbnail for %s: %v", img.Filename, err)
			continue
		}

		ui.thumbMutex.Lock()
		// Check if we need to evict old entries (LRU)
		if len(ui.thumbnailCache) >= ui.maxThumbnails && len(ui.thumbOrder) > 0 {
			// Remove oldest entry
			oldest := ui.thumbOrder[0]
			delete(ui.thumbnailCache, oldest)
			ui.thumbOrder = ui.thumbOrder[1:]
		}

		ui.thumbnailCache[img.Path] = thumb
		ui.thumbOrder = append(ui.thumbOrder, img.Path)
		ui.thumbMutex.Unlock()

		// Refresh list every 5 images to show progress
		if i%5 == 0 {
			ui.refreshListAsync()
		}
	}

	Info("Thumbnail preloading complete, cached %d", len(ui.thumbnailCache))
	ui.refreshListAsync()
}

// refreshListAsync refreshes the list safely from a goroutine
func (ui *UI) refreshListAsync() {
	ui.listMutex.Lock()
	ui.listData = ui.processor.GetTasks()
	count := len(ui.listData)
	ui.listMutex.Unlock()

	if count > 0 {
		ui.imageList.Show()
		ui.emptyLabel.Hide()
	} else {
		ui.imageList.Hide()
		ui.emptyLabel.Show()
	}

	ui.imageList.Refresh()
}

func (ui *UI) onClearList() {
	if ui.processor.IsRunning() {
		dialog.ShowInformation("提示", "请先停止处理再清空列表", ui.window)
		return
	}

	dialog.ShowConfirm("确认清空", "确定要清空图片列表吗?", func(confirmed bool) {
		if confirmed {
			ui.processor.ClearTasks()
			ui.thumbMutex.Lock()
			ui.thumbnailCache = make(map[string]image.Image)
			ui.thumbOrder = make([]string, 0)
			ui.thumbMutex.Unlock()
			// Clear selection map
			ui.selectedMap = make(map[int]bool)
			ui.refreshList()
			ui.btnStart.Disable()
			ui.btnClear.Disable()
			ui.updateStatusBar()
		}
	}, ui.window)
}

func (ui *UI) onSelectAll() {
	ui.listMutex.RLock()
	for i := range ui.listData {
		ui.selectedMap[i] = true
		ui.processor.SetTaskSelected(i, true)
	}
	ui.listMutex.RUnlock()
	ui.imageList.Refresh()
	ui.updateStatusBar()
}

func (ui *UI) onDeselectAll() {
	ui.listMutex.RLock()
	for i := range ui.listData {
		ui.selectedMap[i] = false
		ui.processor.SetTaskSelected(i, false)
	}
	ui.listMutex.RUnlock()
	ui.imageList.Refresh()
	ui.updateStatusBar()
}

func (ui *UI) onStart() {
	if ui.config.APIKey == "" {
		dialog.ShowInformation("提示", "请先在设置中配置 API Key", ui.window)
		ui.onSettings()
		return
	}

	if err := ui.config.EnsureOutputDir(); err != nil {
		dialog.ShowError(fmt.Errorf("无法创建输出目录: %v", err), ui.window)
		return
	}

	selectedCount := ui.processor.GetSelectedCount()
	if selectedCount == 0 {
		dialog.ShowInformation("提示", "请至少选择一个图片进行转换", ui.window)
		return
	}

	dialog.ShowConfirm("确认转换",
		fmt.Sprintf("即将开始转换 %d 个选中的图片\n\n输出目录: %s\n并发数: %d\n\n是否继续?",
			selectedCount, ui.config.OutputDir, ui.config.Concurrency),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			ui.startProcessing()
		}, ui.window)
}

func (ui *UI) startProcessing() {
	ui.isProcessing = true
	ui.btnStart.Hide()
	ui.btnStop.Show()
	ui.btnOpenFolder.Disable()
	ui.btnAddImages.Disable()
	ui.btnClear.Disable()
	ui.progressBar.Show()
	ui.progressBar.SetValue(0)
	ui.progressLabel.SetText("0%")
	ui.statusLabel.SetText("正在处理...")

	Info("Starting processing %d tasks", len(ui.listData))
	ui.processor.Start()
}

func (ui *UI) onStop() {
	dialog.ShowConfirm("确认停止", "确定要停止处理吗?", func(confirmed bool) {
		if confirmed {
			ui.processor.Stop()
			ui.stopProcessing("已停止")
		}
	}, ui.window)
}

func (ui *UI) stopProcessing(status string) {
	if !ui.isProcessing {
		return
	}
	ui.isProcessing = false

	ui.btnStop.Hide()
	ui.btnStart.Show()
	ui.btnOpenFolder.Enable()
	ui.btnAddImages.Enable()
	ui.btnClear.Enable()

	// Set progress to 100% and keep it visible
	ui.progressBar.SetValue(1.0)
	ui.progressLabel.SetText("100%")
	ui.statusLabel.SetText(status)

	// Refresh balance after processing
	go ui.queryBalance()

	// Show completion dialog with option to open output folder
	if strings.Contains(status, "完成") {
		_, _, _, done, failed := ui.processor.GetTaskCount()
		if done > 0 {
			dialog.ShowCustomConfirm("处理完成", "打开输出文件夹", "关闭",
				widget.NewLabel(fmt.Sprintf("%s\n\n成功: %d 个, 失败: %d 个\n是否打开输出文件夹?",
					status, done, failed)),
				func(confirmed bool) {
					if confirmed {
						ui.openOutputFolder()
					}
				}, ui.window)
		}
	}

	Info("Processing stopped: %s", status)
}

func (ui *UI) onSettings() {
	// API Key
	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetText(ui.config.APIKey)
	apiKeyEntry.SetPlaceHolder("输入 DMXAPI API Key")

	// Register link
	registerLink := widget.NewHyperlink("点击此处注册获取 API Key", nil)
	registerLink.OnTapped = func() {
		ui.openURL("https://www.dmxapi.cn/register?aff=9jHw")
	}

	apiKeyRow := container.NewVBox(
		apiKeyEntry,
		container.NewHBox(
			widget.NewLabel("没有账号？"),
			registerLink,
		),
	)

	// System Token (for balance query)
	systemTokenEntry := widget.NewPasswordEntry()
	systemTokenEntry.SetText(ui.config.SystemToken)
	systemTokenEntry.SetPlaceHolder("系统令牌（用于余额查询）")

	systemTokenLink := widget.NewHyperlink("获取系统令牌", nil)
	systemTokenLink.OnTapped = func() {
		ui.openURL("https://www.dmxapi.cn/profile")
	}

	systemTokenRow := container.NewVBox(
		systemTokenEntry,
		container.NewHBox(
			widget.NewLabel("在个人中心创建"),
			systemTokenLink,
		),
	)

	// User ID (for token balance query)
	userIDEntry := widget.NewEntry()
	userIDEntry.SetText(ui.config.UserID)
	userIDEntry.SetPlaceHolder("用户ID（用于令牌余额查询）")

	userIDLink := widget.NewHyperlink("查看用户ID", nil)
	userIDLink.OnTapped = func() {
		ui.openURL("https://www.dmxapi.cn/profile")
	}

	userIDRow := container.NewVBox(
		userIDEntry,
		container.NewHBox(
			widget.NewLabel("在个人中心查看"),
			userIDLink,
		),
	)

	// Output directory
	outputDirEntry := widget.NewEntry()
	outputDirEntry.SetText(ui.config.OutputDir)

	outputDirBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
			if err != nil || dir == nil {
				return
			}
			outputDirEntry.SetText(dir.Path())
		}, ui.window)
		if lister, err := storage.ListerForURI(storage.NewFileURI(ui.config.OutputDir)); err == nil {
			fd.SetLocation(lister)
		}
		fd.Show()
	})

	outputDirRow := container.NewBorder(nil, nil, nil, outputDirBtn, outputDirEntry)

	// Concurrency
	concurrencyLabel := widget.NewLabel(fmt.Sprintf("%d", ui.config.Concurrency))
	concurrencySlider := widget.NewSlider(1, 4)
	concurrencySlider.SetValue(float64(ui.config.Concurrency))
	concurrencySlider.OnChanged = func(v float64) {
		concurrencyLabel.SetText(fmt.Sprintf("%d", int(v)))
	}
	concurrencySlider.Step = 1
	concurrencyRow := container.NewBorder(nil, nil, nil, concurrencyLabel, concurrencySlider)

	// Duration
	durationLabel := widget.NewLabel(fmt.Sprintf("%d 秒", ui.config.Duration))
	durationSlider := widget.NewSlider(4, 10)
	durationSlider.SetValue(float64(ui.config.Duration))
	durationSlider.OnChanged = func(v float64) {
		durationLabel.SetText(fmt.Sprintf("%d 秒", int(v)))
	}
	durationSlider.Step = 1
	durationRow := container.NewBorder(nil, nil, nil, durationLabel, durationSlider)

	// Resolution
	resolutionSelect := widget.NewSelect([]string{"540P", "720P", "768P", "1080P"}, func(s string) {})
	resolutionSelect.SetSelected(ui.config.Resolution)

	// Prompt templates
	templateNames := make([]string, len(PromptTemplates))
	for i, t := range PromptTemplates {
		templateNames[i] = t.Name
	}

	templateSelect := widget.NewSelect(templateNames, func(s string) {
		for _, t := range PromptTemplates {
			if t.Name == s && t.Name != "自定义" {
				// Update prompt entry when template selected
			}
		}
	})

	// Find current template
	for _, t := range PromptTemplates {
		if t.Prompt == ui.config.Prompt {
			templateSelect.SetSelected(t.Name)
			break
		}
	}
	if templateSelect.Selected == "" {
		templateSelect.SetSelected("自定义")
	}

	// Prompt
	promptEntry := widget.NewMultiLineEntry()
	promptEntry.SetText(ui.config.Prompt)
	promptEntry.SetMinRowsVisible(3)

	// LLM optimize button
	llmOptimizeBtn := widget.NewButton("🤖 LLM优化", nil)
	llmOptimizeBtn.Importance = widget.LowImportance

	// Optimize status label
	optimizeStatusLabel := widget.NewLabel("")

	templateSelect.OnChanged = func(s string) {
		for _, t := range PromptTemplates {
			if t.Name == s && t.Name != "自定义" {
				promptEntry.SetText(t.Prompt)
				break
			}
		}
	}

	// LLM optimize button click handler
	llmOptimizeBtn.OnTapped = func() {
		if ui.config.APIKey == "" {
			dialog.ShowInformation("提示", "请先配置 API Key", ui.window)
			return
		}
		if promptEntry.Text == "" {
			dialog.ShowInformation("提示", "请先输入提示词", ui.window)
			return
		}

		llmOptimizeBtn.Disable()
		llmOptimizeBtn.SetText("优化中...")
		optimizeStatusLabel.SetText("正在调用 LLM 优化...")
		originalPrompt := promptEntry.Text

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			apiClient := NewAPIClient(ui.config.APIKey)
			optimizedPrompt, err := apiClient.OptimizePrompt(ctx, originalPrompt)

			// Update UI on main thread
			optimizedPromptCopy := optimizedPrompt
			llmOptimizeBtn.Enable()
			llmOptimizeBtn.SetText("🤖 LLM优化")

			if err != nil {
				optimizeStatusLabel.SetText("")
				dialog.ShowError(fmt.Errorf("LLM优化失败: %v", err), ui.window)
				return
			}

			promptEntry.SetText(optimizedPromptCopy)
			optimizeStatusLabel.SetText("✓ 已优化")
			Info("Prompt optimized successfully")
		}()
	}

	// Prompt entry with optimize button
	promptRow := container.NewBorder(nil, nil, nil, llmOptimizeBtn, promptEntry)

	// Theme selection
	themeSelect := widget.NewSelect([]string{"浅色", "深色"}, func(s string) {})
	if ui.config.Theme == "dark" {
		themeSelect.SetSelected("深色")
	} else {
		themeSelect.SetSelected("浅色")
	}

	// Form
	form := dialog.NewForm("设置", "保存", "取消",
		[]*widget.FormItem{
			widget.NewFormItem("API Key", apiKeyRow),
			widget.NewFormItem("系统令牌", systemTokenRow),
			widget.NewFormItem("用户ID", userIDRow),
			widget.NewFormItem("输出目录", outputDirRow),
			widget.NewFormItem("并发数", concurrencyRow),
			widget.NewFormItem("视频时长", durationRow),
			widget.NewFormItem("分辨率", resolutionSelect),
			widget.NewFormItem("提示词模板", templateSelect),
			widget.NewFormItem("提示词", promptRow),
			widget.NewFormItem("", optimizeStatusLabel),
			widget.NewFormItem("主题", themeSelect),
		},
		func(confirmed bool) {
			if !confirmed {
				return
			}
			ui.config.APIKey = apiKeyEntry.Text
			ui.config.SystemToken = systemTokenEntry.Text
			ui.config.UserID = userIDEntry.Text
			ui.config.OutputDir = outputDirEntry.Text
			ui.config.Concurrency = int(concurrencySlider.Value)
			ui.config.Duration = int(durationSlider.Value)
			ui.config.Resolution = resolutionSelect.Selected
			ui.config.Prompt = promptEntry.Text

			// Update theme
			newTheme := "light"
			if themeSelect.Selected == "深色" {
				newTheme = "dark"
			}
			themeChanged := ui.config.Theme != newTheme
			ui.config.Theme = newTheme

			// Save configuration
			if err := ui.config.Save(); err != nil {
				Warn("Failed to save config: %v", err)
			}

			// Update prompt display
			ui.updatePromptDisplay()

			// Apply theme change immediately
			if themeChanged {
				ui.app.Settings().SetTheme(newWindowsTheme(newTheme == "dark"))
			}

			if ui.config.APIKey == "" {
				dialog.ShowInformation("提示", "API Key 不能为空", ui.window)
			}
		}, ui.window)

	form.Resize(fyne.NewSize(550, 550))
	form.Show()
}

// onHelp opens the help documentation
func (ui *UI) onHelp() {
	// Try to open README.md in the same directory as the executable
	exePath, err := os.Executable()
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}

	readmePath := filepath.Join(filepath.Dir(exePath), "README.md")

	// Check if README.md exists
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		dialog.ShowInformation("提示", "未找到 README.md 文件\n\n请确保 README.md 与程序在同一目录下。", ui.window)
		return
	}

	// Open with system default application
	cmd := exec.Command("cmd", "/c", "start", "", readmePath)
	cmd.Start()
}

// updatePromptDisplay updates the prompt label with current prompt from config
func (ui *UI) updatePromptDisplay() {
	if ui.promptLabel == nil {
		return
	}
	prompt := ui.config.Prompt
	if prompt == "" {
		ui.promptLabel.SetText("")
		return
	}
	// Truncate if too long for display
	if len([]rune(prompt)) > 60 {
		prompt = string([]rune(prompt)[:57]) + "..."
	}
	ui.promptLabel.SetText("💡 提示词: " + prompt)
}

func (ui *UI) refreshList() {
	ui.listMutex.Lock()
	ui.listData = ui.processor.GetTasks()

	// Apply filter
	if ui.filterText != "" {
		var filtered []*Task
		for _, task := range ui.listData {
			if strings.Contains(strings.ToLower(task.Image.Filename), ui.filterText) {
				filtered = append(filtered, task)
			}
		}
		ui.listData = filtered
	}

	count := len(ui.listData)
	ui.listMutex.Unlock()

	Info("refreshList: %d items", count)

	// Check if imageList is initialized (may be called during setup)
	if ui.imageList == nil {
		return
	}

	if count > 0 {
		ui.imageList.Show()
		ui.emptyLabel.Hide()
	} else {
		ui.imageList.Hide()
		ui.emptyLabel.Show()
	}

	// Force list to recalculate item count and refresh all items
	ui.imageList.Refresh()
}

// sortTasks sorts the task list based on current sort selection
func (ui *UI) sortTasks() {
	ui.listMutex.Lock()
	defer ui.listMutex.Unlock()

	tasks := ui.processor.GetTasks()

	switch ui.sortSelect.Selected {
	case "按文件名":
		sort.Slice(tasks, func(i, j int) bool {
			if ui.sortAscending {
				return tasks[i].Image.Filename < tasks[j].Image.Filename
			}
			return tasks[i].Image.Filename > tasks[j].Image.Filename
		})
	case "按状态":
		stateOrder := map[TaskState]int{
			StateFailed:     0,
			StateProcessing: 1,
			StatePending:    2,
			StateDone:       3,
		}
		sort.Slice(tasks, func(i, j int) bool {
			orderI := stateOrder[tasks[i].State]
			orderJ := stateOrder[tasks[j].State]
			if ui.sortAscending {
				return orderI < orderJ
			}
			return orderI > orderJ
		})
	case "按尺寸":
		sort.Slice(tasks, func(i, j int) bool {
			sizeI := tasks[i].Image.Width * tasks[i].Image.Height
			sizeJ := tasks[j].Image.Width * tasks[j].Image.Height
			if ui.sortAscending {
				return sizeI < sizeJ
			}
			return sizeI > sizeJ
		})
	default:
		// Default order - no sorting needed
		return
	}

	// Update processor's task list
	ui.processor.SetTasks(tasks)
}

// queryBalance queries the API account balance and token balance
func (ui *UI) queryBalance() {
	if ui.config.SystemToken == "" {
		ui.balanceLabel.SetText("请先配置系统令牌")
		return
	}
	if ui.config.UserID == "" {
		ui.balanceLabel.SetText("请先配置用户ID")
		return
	}

	ui.balanceLabel.SetText("查询中...")

	apiClient := NewAPIClient(ui.config.APIKey)
	ctx := context.Background()

	// Query token balance (remaining count)
	remainCount, err := apiClient.GetTokenBalance(ctx, ui.config.SystemToken, ui.config.UserID)
	if err != nil {
		ui.balanceLabel.SetText(fmt.Sprintf("查询失败: %v", err))
		Error("Failed to query token balance: %v", err)
		return
	}

	// Query account balance
	info, err := apiClient.GetBalance(ctx, ui.config.SystemToken, ui.config.UserID)
	if err != nil {
		// Show token count even if account balance fails
		ui.balanceLabel.SetText(fmt.Sprintf("令牌: %d次 | 余额: 查询失败", remainCount))
		Error("Failed to query account balance: %v", err)
		return
	}

	ui.balanceLabel.SetText(fmt.Sprintf("令牌剩余次数: %d次 | 余额: %.2f元", remainCount, info.Balance))
	Info("Token balance: remaining=%d times, balance=%.2f CNY", remainCount, info.Balance)
}

// balanceRefreshLoop periodically refreshes balance during processing
func (ui *UI) balanceRefreshLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Only auto-refresh if processing is running
		if ui.processor.IsRunning() {
			ui.queryBalance()
		}
	}
}

func (ui *UI) updateStatusBar() {
	total, pending, processing, done, failed := ui.processor.GetTaskCount()

	// Count selected from local map
	selected := 0
	for _, s := range ui.selectedMap {
		if s {
			selected++
		}
	}

	ui.totalLabel.SetText(fmt.Sprintf("总计: %d (选中: %d)", total, selected))
	ui.completedLabel.SetText(fmt.Sprintf("完成: %d", done))
	ui.failedLabel.SetText(fmt.Sprintf("失败: %d", failed))

	// Update progress bar and percentage
	if total > 0 {
		progress := float64(done+failed) / float64(total)
		ui.progressBar.SetValue(progress)
		ui.progressLabel.SetText(fmt.Sprintf("%.0f%%", progress*100))
	}

	// Update status text
	if ui.processor.IsRunning() {
		ui.statusLabel.SetText(fmt.Sprintf("处理中... (等待: %d, 处理中: %d)", pending, processing))
	} else if ui.isProcessing {
		if done == total {
			ui.stopProcessing("全部完成!")
		} else if done+failed == total {
			ui.stopProcessing(fmt.Sprintf("处理完成 (%d 个失败)", failed))
		} else {
			ui.stopProcessing("处理已停止")
		}
	} else if total == 0 {
		ui.statusLabel.SetText("就绪")
	}
}

func (ui *UI) listItemCount() int {
	ui.listMutex.RLock()
	defer ui.listMutex.RUnlock()
	count := len(ui.listData)
	Debug("listItemCount: %d", count)
	return count
}

// contextListItem is a container that supports right-click context menu
type contextListItem struct {
	widget.BaseWidget
	container *fyne.Container
	ui        *UI
	index     int
}

// CreateRenderer implements fyne.Widget interface
func (c *contextListItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.container)
}

// MouseDown handles right-click to show context menu
func (c *contextListItem) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonSecondary {
		c.ui.showContextMenu(c.index, e.AbsolutePosition)
	}
}

// MouseUp is required by desktop.Mouseable interface
func (c *contextListItem) MouseUp(*desktop.MouseEvent) {}

// Tapped is required by fyne.Tappable interface
func (c *contextListItem) Tapped(*fyne.PointEvent) {}

func (ui *UI) showContextMenu(index int, pos fyne.Position) {
	ui.listMutex.RLock()
	if index >= len(ui.listData) {
		ui.listMutex.RUnlock()
		return
	}
	task := ui.listData[index]
	selected := ui.selectedMap[index]
	ui.listMutex.RUnlock()

	// Create context menu items
	var items []*fyne.MenuItem

	// Toggle selection
	if selected {
		items = append(items, fyne.NewMenuItem("取消选中", func() {
			ui.toggleSelection(index, false)
		}))
	} else {
		items = append(items, fyne.NewMenuItem("选中", func() {
			ui.toggleSelection(index, true)
		}))
	}

	// Convert single image (only if pending)
	if task.State == StatePending {
		items = append(items, fyne.NewMenuItemSeparator())
		items = append(items, fyne.NewMenuItem("转换此图片", func() {
			ui.convertSingleImage(index)
		}))
	}

	// Open video (only if done)
	if task.State == StateDone && task.OutputPath != "" {
		items = append(items, fyne.NewMenuItemSeparator())
		items = append(items, fyne.NewMenuItem("打开视频", func() {
			ui.openVideoFile(task.OutputPath)
		}))
		items = append(items, fyne.NewMenuItem("打开视频目录", func() {
			ui.openVideoFolder(task.OutputPath)
		}))
	}

	// Retry (only if failed)
	if task.State == StateFailed {
		items = append(items, fyne.NewMenuItemSeparator())
		items = append(items, fyne.NewMenuItem("重试", func() {
			ui.retryTask(index)
		}))
	}

	// Delete from list (only if not processing)
	if !ui.processor.IsRunning() && task.State != StateProcessing && task.State != StateEncoding && task.State != StateSubmitting && task.State != StateDownloading {
		items = append(items, fyne.NewMenuItemSeparator())
		items = append(items, fyne.NewMenuItem("从列表删除", func() {
			ui.removeTask(index)
		}))
	}

	menu := fyne.NewMenu("", items...)
	ui.contextMenu = widget.NewPopUpMenu(menu, ui.window.Canvas())
	ui.contextMenu.ShowAtPosition(pos)
}

func (ui *UI) toggleSelection(index int, selected bool) {
	ui.listMutex.Lock()
	ui.selectedMap[index] = selected
	ui.listMutex.Unlock()
	ui.processor.SetTaskSelected(index, selected)
	ui.imageList.Refresh()
	ui.updateStatusBar()
}

func (ui *UI) convertSingleImage(index int) {
	if ui.config.APIKey == "" {
		dialog.ShowInformation("提示", "请先在设置中配置 API Key", ui.window)
		return
	}

	if err := ui.config.EnsureOutputDir(); err != nil {
		dialog.ShowError(fmt.Errorf("无法创建输出目录: %v", err), ui.window)
		return
	}

	// Deselect all except this one
	ui.listMutex.Lock()
	for i := range ui.selectedMap {
		ui.selectedMap[i] = (i == index)
		ui.processor.SetTaskSelected(i, i == index)
	}
	ui.listMutex.Unlock()

	ui.imageList.Refresh()
	ui.updateStatusBar()
	ui.startProcessing()
}

func (ui *UI) openVideoFolder(path string) {
	dir := filepath.Dir(path)
	cmd := exec.Command("explorer", dir)
	cmd.Start()
}

// openOutputFolder opens the output directory in explorer
func (ui *UI) openOutputFolder() {
	outputDir := ui.config.OutputDir
	// Ensure directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		dialog.ShowError(fmt.Errorf("输出目录不存在: %s", outputDir), ui.window)
		return
	}
	cmd := exec.Command("explorer", outputDir)
	cmd.Start()
}

func (ui *UI) removeTask(index int) {
	ui.processor.RemoveTask(index)

	// Update selectedMap
	ui.listMutex.Lock()
	newMap := make(map[int]bool)
	for i, selected := range ui.selectedMap {
		if i < index {
			newMap[i] = selected
		} else if i > index {
			newMap[i-1] = selected
		}
	}
	ui.selectedMap = newMap
	ui.listMutex.Unlock()

	ui.refreshList()
	ui.updateStatusBar()

	// Disable buttons if list is empty
	ui.listMutex.RLock()
	count := len(ui.listData)
	ui.listMutex.RUnlock()

	if count == 0 {
		ui.btnStart.Disable()
		ui.btnClear.Disable()
	}
}

func (ui *UI) listItemCreate() fyne.CanvasObject {
	Debug("listItemCreate called")

	// Checkbox for selection
	selectBtn := widget.NewButtonWithIcon("", theme.Icon(theme.IconNameConfirm), nil)
	selectBtn.Importance = widget.LowImportance

	// Thumbnail with placeholder
	thumb := canvas.NewImageFromImage(GetPlaceholderImage())
	thumb.FillMode = canvas.ImageFillContain
	thumb.SetMinSize(fyne.NewSize(80, 60))

	// Left side: select button + thumbnail
	leftBox := container.NewVBox(selectBtn, thumb)

	// Info labels
	filenameLabel := widget.NewLabel("filename.jpg")
	filenameLabel.Importance = widget.MediumImportance
	dimensionsLabel := widget.NewLabel("1920 x 1080")
	dimensionsLabel.Importance = widget.LowImportance
	stateLabel := widget.NewLabel("等待中")

	// Progress
	progress := widget.NewProgressBar()
	progress.Hide()

	// Open video button
	openBtn := widget.NewButtonWithIcon("打开视频", theme.MediaPlayIcon(), nil)
	openBtn.Hide()

	// Retry button
	retryBtn := widget.NewButtonWithIcon("重试", theme.ViewRefreshIcon(), nil)
	retryBtn.Hide()

	// Right side buttons (vertical)
	rightBox := container.NewVBox(openBtn, retryBtn)

	// Info section
	infoBox := container.NewVBox(
		filenameLabel,
		dimensionsLabel,
		stateLabel,
		progress,
	)

	// Main container: left | info | right
	item := container.NewBorder(nil, nil, leftBox, rightBox, infoBox)

	// Store widget references
	ui.widgetsMutex.Lock()
	ui.listWidgets[len(ui.listWidgets)] = &listItemWidgets{
		Container:       item,
		selectBtn:       selectBtn,
		thumb:           thumb,
		filenameLabel:   filenameLabel,
		dimensionsLabel: dimensionsLabel,
		stateLabel:      stateLabel,
		progress:        progress,
		openBtn:         openBtn,
		retryBtn:        retryBtn,
	}
	ui.widgetsMutex.Unlock()

	Debug("listItemCreate: returning container")
	// Wrap in contextListItem for right-click menu support
	ctxItem := &contextListItem{
		container: item,
		ui:        ui,
	}
	ctxItem.ExtendBaseWidget(ctxItem)
	return ctxItem
}

// listItemWidgets holds references to all widgets in a list item
type listItemWidgets struct {
	*fyne.Container
	selectBtn       *widget.Button
	thumb           *canvas.Image
	filenameLabel   *widget.Label
	dimensionsLabel *widget.Label
	stateLabel      *widget.Label
	progress        *widget.ProgressBar
	openBtn         *widget.Button
	retryBtn        *widget.Button
}

func (ui *UI) listItemUpdate(index int, obj fyne.CanvasObject) {
	Debug("listItemUpdate called: index=%d", index)

	ui.listMutex.RLock()
	if index >= len(ui.listData) {
		ui.listMutex.RUnlock()
		Debug("listItemUpdate: index %d out of range (len=%d)", index, len(ui.listData))
		return
	}
	task := ui.listData[index]
	imagePath := task.Image.Path
	ui.listMutex.RUnlock()

	Debug("listItemUpdate: index=%d, file=%s", index, task.Image.Filename)

	// Update index in contextListItem for context menu and get inner container
	var innerContainer *fyne.Container
	switch v := obj.(type) {
	case *contextListItem:
		v.index = index
		innerContainer = v.container
		Debug("listItemUpdate: got contextListItem, container has %d objects", len(innerContainer.Objects))
	case *fyne.Container:
		innerContainer = v
		Debug("listItemUpdate: got fyne.Container, has %d objects", len(innerContainer.Objects))
	default:
		Debug("listItemUpdate: type assertion failed, obj is %T", obj)
		return
	}

	// Extract widgets by traversing all objects
	var selectBtn *widget.Button
	var thumb *canvas.Image
	var filenameLabel, dimensionsLabel, stateLabel *widget.Label
	var progress *widget.ProgressBar
	var openBtn, retryBtn *widget.Button

	// Helper function to find widgets recursively
	var findWidgets func(obj fyne.CanvasObject)
	findWidgets = func(obj fyne.CanvasObject) {
		switch w := obj.(type) {
		case *widget.Button:
			if w.Text == "" || w.Text == " " {
				selectBtn = w // Icon-only button is select button
			} else if strings.Contains(w.Text, "打开视频") {
				openBtn = w
			} else if strings.Contains(w.Text, "重试") || strings.Contains(w.Text, "再次转换") {
				retryBtn = w
			}
		case *canvas.Image:
			thumb = w
		case *widget.Label:
			if filenameLabel == nil {
				filenameLabel = w
			} else if dimensionsLabel == nil && strings.Contains(w.Text, "x") {
				dimensionsLabel = w
			} else if stateLabel == nil {
				stateLabel = w
			}
		case *widget.ProgressBar:
			progress = w
		case *fyne.Container:
			for _, child := range w.Objects {
				findWidgets(child)
			}
		}
	}
	findWidgets(innerContainer)

	// Update text labels
	Debug("listItemUpdate: found widgets - filenameLabel: %v, dimensionsLabel: %v, stateLabel: %v, thumb: %v, selectBtn: %v",
		filenameLabel != nil, dimensionsLabel != nil, stateLabel != nil, thumb != nil, selectBtn != nil)
	if filenameLabel != nil {
		filenameLabel.SetText(task.Image.Filename)
	}
	if dimensionsLabel != nil {
		dimensionsLabel.SetText(fmt.Sprintf("%d x %d", task.Image.Width, task.Image.Height))
	}

	// Get current selection state
	ui.listMutex.RLock()
	selected := ui.selectedMap[index]
	ui.listMutex.RUnlock()

	// Update select button
	if selectBtn != nil {
		if selected {
			selectBtn.SetIcon(theme.Icon(theme.IconNameConfirm))
			selectBtn.Importance = widget.HighImportance
		} else {
			selectBtn.SetIcon(theme.Icon(theme.IconNameNavigateNext))
			selectBtn.Importance = widget.LowImportance
		}

		// Set button click handler
		btnIndex := index
		selectBtn.OnTapped = func() {
			Info("Select button %d tapped", btnIndex)
			ui.listMutex.Lock()
			ui.selectedMap[btnIndex] = !ui.selectedMap[btnIndex]
			selected := ui.selectedMap[btnIndex]
			ui.listMutex.Unlock()
			ui.processor.SetTaskSelected(btnIndex, selected)
			ui.imageList.Refresh()
			ui.updateStatusBar()
		}
	}

	// Update thumbnail if available
	ui.thumbMutex.RLock()
	if cachedThumb, ok := ui.thumbnailCache[imagePath]; ok && thumb != nil {
		thumb.Image = cachedThumb
		thumb.Refresh()
	}
	ui.thumbMutex.RUnlock()

	// Reset visibility
	if progress != nil {
		progress.Hide()
	}
	if openBtn != nil {
		openBtn.Hide()
	}
	if retryBtn != nil {
		retryBtn.Hide()
	}

	// Update state
	switch task.State {
	case StatePending:
		if stateLabel != nil {
			stateLabel.SetText("等待中")
			stateLabel.Importance = widget.LowImportance
		}
	case StateEncoding:
		if stateLabel != nil {
			stateLabel.SetText("编码中...")
			stateLabel.Importance = widget.MediumImportance
		}
	case StateSubmitting:
		if stateLabel != nil {
			stateLabel.SetText("提交中...")
			stateLabel.Importance = widget.MediumImportance
		}
	case StateProcessing:
		if stateLabel != nil {
			stateLabel.SetText("处理中...")
			stateLabel.Importance = widget.HighImportance
		}
		if progress != nil {
			progress.Show()
			progress.SetValue(0.5) // API doesn't provide percentage, show 50%
		}
	case StateDownloading:
		if stateLabel != nil {
			stateLabel.SetText("下载中...")
			stateLabel.Importance = widget.HighImportance
		}
		if progress != nil {
			progress.Show()
			progress.SetValue(0.8) // Downloading, show 80%
		}
	case StateDone:
		if stateLabel != nil {
			stateLabel.SetText("已完成")
			stateLabel.Importance = widget.SuccessImportance
		}
		if progress != nil {
			progress.SetValue(1.0)
			progress.Hide()
		}
		if task.OutputPath != "" && openBtn != nil {
			openBtn.Show()
			videoPath := task.OutputPath
			openBtn.OnTapped = func() {
				ui.openVideoFile(videoPath)
			}
		}
		// Show retry button for re-conversion
		if retryBtn != nil {
			retryBtn.Show()
			retryBtn.SetText("再次转换")
			taskIndex := index
			retryBtn.OnTapped = func() {
				ui.retryTask(taskIndex)
			}
		}
	case StateFailed:
		errorMsg := "未知错误"
		if task.Error != nil {
			errorMsg = task.Error.Error()
		}
		if stateLabel != nil {
			stateLabel.SetText(fmt.Sprintf("失败: %s", errorMsg))
			stateLabel.Importance = widget.DangerImportance
		}
		if progress != nil {
			progress.Hide()
		}
		if retryBtn != nil {
			retryBtn.Show()
			retryBtn.SetText("重试")
			taskIndex := index
			retryBtn.OnTapped = func() {
				ui.retryTask(taskIndex)
			}
		}
	case StateCancelled:
		if stateLabel != nil {
			stateLabel.SetText("已取消")
			stateLabel.Importance = widget.LowImportance
		}
	}

	Debug("listItemUpdate: done")
}

// openVideoFile opens the video file with the system default player
func (ui *UI) openVideoFile(path string) {
	Info("Opening video: %s", path)
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.Start()
}

// openURL opens a URL in the system default browser
func (ui *UI) openURL(url string) {
	Info("Opening URL: %s", url)
	cmd := exec.Command("cmd", "/c", "start", "", url)
	cmd.Start()
}

// retryTask retries a failed or completed task
func (ui *UI) retryTask(index int) {
	ui.listMutex.Lock()
	if index >= len(ui.listData) {
		ui.listMutex.Unlock()
		return
	}
	task := ui.listData[index]
	ui.listMutex.Unlock()

	// Allow retry for both failed and completed tasks
	if task.State != StateFailed && task.State != StateDone {
		return
	}

	Info("Retrying task: %s", task.Image.Filename)

	// Reset task state
	task.State = StatePending
	task.Error = nil
	task.OutputPath = ""

	// Ensure task is selected for processing
	ui.listMutex.Lock()
	ui.selectedMap[index] = true
	ui.listMutex.Unlock()
	ui.processor.SetTaskSelected(index, true)

	// Refresh UI
	ui.refreshList()
	ui.updateStatusBar()

	// Start processing if not already running
	if !ui.processor.IsRunning() {
		ui.processor.Start()
	}
}

func (ui *UI) eventListener() {
	for range ui.processor.Events() {
		// Must refresh UI on main thread
		ui.imageList.Refresh()
		ui.updateStatusBar()
	}
}

func (ui *UI) updateLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// Only update status bar, not the list
		// List updates happen via events or explicit user actions
		ui.updateStatusBar()
	}
}
