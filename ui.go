//go:build gui
// +build gui

package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
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
}

func newWindowsTheme() fyne.Theme {
	return &windowsTheme{theme.DefaultTheme()}
}

func (t *windowsTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
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
	}
	return t.Theme.Color(name, variant)
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
	btnOpenFolder *widget.Button
	btnAddImages  *widget.Button
	btnClear      *widget.Button
	btnSettings   *widget.Button
	btnSelectAll  *widget.Button
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
	selectedMap   map[int]bool // Track selection state by index
	checkboxes    map[int]*widget.Check // Store checkbox references
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

	// Containers
	mainContainer *fyne.Container

	// State
	isProcessing bool

	// Context menu
	contextMenu      *widget.PopUpMenu
	contextMenuIndex int // Index of the item that was right-clicked

	// Drag and drop
	dropWell *widget.Label
}

// NewUI creates the main application UI
func NewUI(app fyne.App, processor *Processor, config *Config) *UI {
	app.Settings().SetTheme(newWindowsTheme())

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

	// Image list
	ui.emptyLabel = widget.NewLabel("请点击「选择文件夹」或「添加图片」来加载图片\n\n支持格式: JPG, PNG, WEBP\n\n也可以直接拖拽图片到此处")
	ui.emptyLabel.Alignment = fyne.TextAlignCenter
	ui.emptyLabel.Importance = widget.LowImportance

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
	developerLabel.Importance = widget.LowImportance

	statusBar := container.NewHBox(
		ui.statusLabel,
		layout.NewSpacer(),
		ui.totalLabel,
		widget.NewSeparator(),
		ui.completedLabel,
		widget.NewSeparator(),
		ui.failedLabel,
		widget.NewSeparator(),
		developerLabel,
	)

	// Progress bar row
	progressRow := container.NewBorder(nil, nil, nil, nil, ui.progressBar)

	// Main layout
	headerSection := container.NewVBox(
		ui.titleLabel,
		widget.NewSeparator(),
		toolbarRow,
		actionRow,
		widget.NewSeparator(),
	)

	footerSection := container.NewVBox(
		widget.NewSeparator(),
		progressRow,
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
	ui.progressBar.Hide()
	ui.progressLabel.SetText("")
	ui.statusLabel.SetText(status)

	Info("Processing stopped: %s", status)
}

func (ui *UI) onSettings() {
	// API Key
	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetText(ui.config.APIKey)
	apiKeyEntry.SetPlaceHolder("输入 DMXAPI API Key")

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

	templateSelect.OnChanged = func(s string) {
		for _, t := range PromptTemplates {
			if t.Name == s && t.Name != "自定义" {
				promptEntry.SetText(t.Prompt)
				break
			}
		}
	}

	// Form
	form := dialog.NewForm("设置", "保存", "取消",
		[]*widget.FormItem{
			widget.NewFormItem("API Key", apiKeyEntry),
			widget.NewFormItem("输出目录", outputDirRow),
			widget.NewFormItem("并发数", concurrencyRow),
			widget.NewFormItem("视频时长", durationRow),
			widget.NewFormItem("分辨率", resolutionSelect),
			widget.NewFormItem("提示词模板", templateSelect),
			widget.NewFormItem("提示词", promptEntry),
		},
		func(confirmed bool) {
			if !confirmed {
				return
			}
			ui.config.APIKey = apiKeyEntry.Text
			ui.config.OutputDir = outputDirEntry.Text
			ui.config.Concurrency = int(concurrencySlider.Value)
			ui.config.Duration = int(durationSlider.Value)
			ui.config.Resolution = resolutionSelect.Selected
			ui.config.Prompt = promptEntry.Text

			// Save configuration
			if err := ui.config.Save(); err != nil {
				Warn("Failed to save config: %v", err)
			}

			if ui.config.APIKey == "" {
				dialog.ShowInformation("提示", "API Key 不能为空", ui.window)
			}
		}, ui.window)

	form.Resize(fyne.NewSize(550, 450))
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

func (ui *UI) refreshList() {
	ui.listMutex.Lock()
	ui.listData = ui.processor.GetTasks()
	count := len(ui.listData)
	ui.listMutex.Unlock()

	Info("refreshList: %d items", count)

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
	Info("listItemCount: %d", count)
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
	dir := strings.TrimSuffix(path, "/"+strings.Split(path, "/")[len(strings.Split(path, "/"))-1])
	cmd := exec.Command("explorer", dir)
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
	Info("listItemCreate called")

	// Checkbox for selection
	selectBtn := widget.NewButtonWithIcon("", theme.Icon(theme.IconNameConfirm), nil)
	selectBtn.Importance = widget.LowImportance

	// Thumbnail
	thumb := canvas.NewImageFromImage(nil)
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
		Container:        item,
		selectBtn:        selectBtn,
		thumb:            thumb,
		filenameLabel:    filenameLabel,
		dimensionsLabel:  dimensionsLabel,
		stateLabel:       stateLabel,
		progress:         progress,
		openBtn:          openBtn,
		retryBtn:         retryBtn,
	}
	ui.widgetsMutex.Unlock()

	Info("listItemCreate: returning container")
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
	Info("listItemUpdate called: index=%d", index)

	ui.listMutex.RLock()
	if index >= len(ui.listData) {
		ui.listMutex.RUnlock()
		Info("listItemUpdate: index %d out of range (len=%d)", index, len(ui.listData))
		return
	}
	task := ui.listData[index]
	imagePath := task.Image.Path
	ui.listMutex.RUnlock()

	Info("listItemUpdate: index=%d, file=%s", index, task.Image.Filename)

	// Update index in contextListItem for context menu and get inner container
	var innerContainer *fyne.Container
	switch v := obj.(type) {
	case *contextListItem:
		v.index = index
		innerContainer = v.container
		Info("listItemUpdate: got contextListItem, container has %d objects", len(innerContainer.Objects))
	case *fyne.Container:
		innerContainer = v
		Info("listItemUpdate: got fyne.Container, has %d objects", len(innerContainer.Objects))
	default:
		Info("listItemUpdate: type assertion failed, obj is %T", obj)
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
	Info("listItemUpdate: found widgets - filenameLabel: %v, dimensionsLabel: %v, stateLabel: %v, thumb: %v, selectBtn: %v",
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

	Info("listItemUpdate: done")
}

// openVideoFile opens the video file with the system default player
func (ui *UI) openVideoFile(path string) {
	Info("Opening video: %s", path)
	cmd := exec.Command("cmd", "/c", "start", "", path)
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
