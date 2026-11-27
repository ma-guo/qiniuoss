package main

import (
	"fmt"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// File-table column widths, shared by the header and every row so the
// columns line up.
const (
	colCheck  = 22
	colIcon   = 16
	colGap    = 10
	colSize   = 76
	colType   = 110
	colTime   = 130
	colMenu   = 30
	listInset = 14
)

// Responsive column visibility, read from the window width.
func showTypeCol(cx *core.Scope) bool { return cx.Env().Window().Width >= 900 }
func showTimeCol(cx *core.Scope) bool { return cx.Env().Window().Width >= 760 }

// colIf shows a column node only while cond holds.
func colIf(cond func() bool, node core.Node) core.Node {
	return core.Show{When: cond,
		Then: func(*core.Scope) core.Node { return node },
		Else: func(*core.Scope) core.Node { return core.View{} }}
}

func (s *appState) browseView(cx *core.Scope) core.Node {
	return core.View{
		Style:    core.Style{Direction: core.Column, FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
		Children: []core.Node{s.browseToolbar(cx), s.selectionBar(cx), s.fileList(cx)},
	}
}

func (s *appState) browseToolbar(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	compact := cx.Env().Window().Width < 640
	filterW := core.Points(220)
	pathMax := core.Points(360)
	if compact {
		filterW = 130
		pathMax = 150
	}
	path := core.Text{ContentFunc: func() string {
		root := "/" + s.bucket.Get()
		if p := s.prefix.Get(); p != "" {
			return root + "/" + strings.TrimSuffix(p, "/")
		}
		return root
	}, MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
		Style: ui.TypeLabel.Merge(core.Style{MaxWidth: core.Pt(pathMax), FlexShrink: 1, MinWidth: core.Pt(0)}), StyleFunc: ui.Colored(colors.Text)}

	var newBtn, upBtn core.Node
	if compact {
		newBtn = ui.IconButton{Icon: lucide.FolderPlus, Label: s.t("新建"), OnPress: func() { s.folderName.Set(""); s.showFolder.Set(true) }}
		upBtn = ui.IconButton{Icon: lucide.Upload, Label: s.t("上传"), Tint: colors.Accent, OnPress: func() { s.beginUpload() }}
	} else {
		newBtn = ui.Button{Text: s.t("新建"), Icon: lucide.FolderPlus, Variant: ui.ButtonOutline, OnPress: func() { s.folderName.Set(""); s.showFolder.Set(true) }}
		upBtn = ui.Button{Text: s.t("上传"), Icon: lucide.Upload, Variant: ui.ButtonPrimary, OnPress: func() { s.beginUpload() }}
	}
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(8, 12),
			Wrap: core.Wrap, BorderWidth: 1, Borders: core.Edges{Bottom: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			ui.IconButton{Icon: lucide.ArrowLeft, Label: s.t("上一级"), OnPress: func() { s.goUp() }, Disabled: func() bool { return s.prefix.Get() == "" }},
			ui.IconButton{Icon: lucide.RefreshCw, Label: s.t("刷新"), OnPress: func() { s.loadList(false) }},
			path,
			core.View{Style: core.Style{FlexGrow: 1}},
			core.View{Style: core.Style{Width: core.Pt(filterW)}, Children: []core.Node{ui.Input{
				Value:       func() string { return s.filter.Get() },
				OnChange:    func(v string) { s.filter.Set(v) },
				Placeholder: s.t("过滤当前目录…"),
			}}},
			newBtn,
			upBtn,
			core.Show{When: func() bool { return len(s.uploads.Get()) > 0 }, Then: func(cx *core.Scope) core.Node {
				if compact {
					return ui.IconButton{Icon: lucide.Upload, Label: s.tf("上传列表 (%d)", s.activeUploads()), OnPress: func() { s.uploadPanel.Set(true) }}
				}
				return ui.Button{TextFunc: func() string { return s.tf("上传列表 (%d)", s.activeUploads()) },
					Icon: lucide.Upload, Variant: ui.ButtonOutline, OnPress: func() { s.uploadPanel.Set(true) }}
			}, Else: func(*core.Scope) core.Node { return core.View{} }},
		},
	}
}

// selAction draws a selection action: a text button on wide screens, a
// square icon button (with a tooltip/label) on compact ones.
func (s *appState) selAction(compact bool, text string, icon func() *core.SVG, variant ui.ButtonVariant, danger bool, disabled func() bool, onPress func()) core.Node {
	if compact {
		var tint func() core.Color
		if danger {
			tint = ui.ColorsOf(s.sc).Danger
		}
		return ui.IconButton{Icon: icon, Label: text, Disabled: disabled, OnPress: onPress, Tint: tint}
	}
	return ui.Button{Text: text, Icon: icon, Variant: variant, Disabled: disabled, OnPress: onPress}
}

func (s *appState) selectionBar(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	compact := cx.Env().Window().Width < 640
	none := func() bool { return len(s.activeKeys()) == 0 }
	single := func() bool { keys := s.activeKeys(); return len(keys) == 1 && !strings.HasSuffix(keys[0], "/") }
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Wrap: core.Wrap, Padding: core.Symmetric(6, 12),
			Background: ui.Alpha(colors.Accent(), 0.08), BorderWidth: 1, Borders: core.Edges{Bottom: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			s.selAction(compact, s.t("预览"), lucide.Eye, ui.ButtonGhost, false, func() bool { return !single() }, func() {
				if keys := s.activeKeys(); len(keys) == 1 && !strings.HasSuffix(keys[0], "/") {
					s.previewObject(object{Key: keys[0]})
				}
			}),
			s.selAction(compact, s.t("下载"), lucide.Download, ui.ButtonGhost, false, none, func() {
				s.requestDownload(s.activeKeys())
			}),
			s.selAction(compact, s.t("复制到"), lucide.Copy, ui.ButtonGhost, false, none, func() { s.showCopyMoveSelected(false) }),
			s.selAction(compact, s.t("移动到"), lucide.ArrowLeftRight, ui.ButtonGhost, false, none, func() { s.showCopyMoveSelected(true) }),
			s.selAction(compact, s.t("复制链接"), lucide.Link, ui.ButtonGhost, false, func() bool { return !single() }, func() {
				keys := s.activeKeys()
				if len(keys) == 1 && !strings.HasSuffix(keys[0], "/") {
					s.copyLink(object{Key: keys[0]})
				}
			}),
			s.selAction(compact, s.t("删除"), lucide.Trash, ui.ButtonDanger, true, none, func() { s.requestDelete(s.activeKeys()) }),
			ui.IconButton{Icon: lucide.X, Label: s.t("清除选择"), Disabled: none, OnPress: func() { s.clearSelected(); s.cursor.Set("") }},
			core.View{Style: core.Style{FlexGrow: 1}},
			core.Text{ContentFunc: func() string {
				if n := s.selectedCount(); n > 0 {
					return s.tf("已勾选 %d 项", n)
				}
				if cur := s.cursor.Get(); cur != "" {
					return baseName(cur)
				}
				return s.t("未选择任何项")
			}, MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
				Style:     ui.TypeLabel.Merge(core.Style{FlexShrink: 1, MinWidth: core.Pt(0), TextAlign: core.TextAlignEnd, MaxWidth: core.Pt(360)}),
				StyleFunc: ui.Colored(colors.Text)},
		},
	}
}

func (s *appState) fileList(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{
		Style:      core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), Direction: core.Column},
		OnFileDrop: func(uris []string) { s.dropFiles(uris) },
		Children: []core.Node{
			s.fileListHeader(cx),
			core.Show{
				When: func() bool { return len(s.visible.Get()) == 0 },
				Then: func(cx *core.Scope) core.Node {
					return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, MinHeight: core.Pt(160)}, Children: []core.Node{
						core.Show{When: func() bool { return s.loading.Get() }, Then: func(cx *core.Scope) core.Node {
							return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
								ui.Spinner{Size: 16},
								core.Text{Content: s.t("加载中…"), Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)},
							}}
						}, Else: func(cx *core.Scope) core.Node {
							return core.Text{ContentFunc: func() string {
								if len(s.accounts.Get()) == 0 {
									return s.t("尚未配置账户，请点击左侧「账户」添加 AccessKey / SecretKey")
								}
								if s.bucket.Get() == "" {
									return s.t("请选择左侧的存储空间")
								}
								return s.t("此目录为空")
							}, Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)}
						}}}}
				},
				Else: func(cx *core.Scope) core.Node {
					return core.Scroll{
						Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
						Children: []core.Node{
							core.Each(
								func() []object { return s.visible.Get() },
								func(o object) string { return o.Key },
								func(row *core.Scope, item func() object, _ func() int) core.Node {
									return s.fileRow(row, item())
								},
							),
							core.Show{When: func() bool { return s.hasNext.Get() }, Then: func(cx *core.Scope) core.Node {
								return core.View{Style: core.Style{AlignItems: core.AlignCenter, Padding: core.All(10)}, Children: []core.Node{
									ui.Button{Text: s.t("加载更多"), Variant: ui.ButtonOutline, BusyLabel: s.t("加载中"),
										Busy: func() bool { return s.loadingMore.Get() }, OnPress: func() { s.loadList(true) }},
								}}
							}},
						},
					}
				},
			},
		},
	}
}

// headText / cellText are table columns. The name is a plain flexing
// text (the pattern the gallery uses); the others are fixed-width and
// never shrink, so the columns line up.
func headText(content string, style core.Style, width core.Points, align core.TextAlign) core.Node {
	if width > 0 {
		style = style.Merge(core.Style{Width: core.Pt(width), FlexShrink: 0})
	}
	if align != 0 {
		style = style.Merge(core.Style{TextAlign: align})
	}
	return core.Text{Content: content, MaxLines: 1, Style: style}
}

func cellText(content string, style core.Style, width core.Points, align core.TextAlign, color func() core.Color) core.Node {
	style = style.Merge(core.Style{Width: core.Pt(width), FlexShrink: 0})
	if align != 0 {
		style = style.Merge(core.Style{TextAlign: align})
	}
	return core.Text{Content: content, MaxLines: 1, EllipsizeMode: core.EllipsizeTail, Style: style, StyleFunc: ui.Colored(color)}
}

func checkCell(child core.Node) core.Node {
	return core.View{
		Style:    core.Style{Width: core.Pt(colCheck), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Children: []core.Node{child},
	}
}

func (s *appState) fileListHeader(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	head := ui.TypeCaption.Merge(core.Style{Color: colors.Muted()})
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: colGap,
			Padding: core.Symmetric(6, listInset), BorderWidth: 1, Borders: core.Edges{Bottom: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			checkCell(ui.Checkbox{
				Checked:       func() bool { return s.allVisibleSelected() },
				Indeterminate: func() bool { return s.someVisibleSelected() && !s.allVisibleSelected() },
				Label:         s.t("全选"),
				OnChange: func(v bool) {
					if v {
						s.selectAllVisible()
					} else {
						s.clearVisibleSelection()
					}
				},
			}),
			core.View{Style: core.Style{Width: core.Pt(colIcon), FlexShrink: 0}},
			core.Text{Content: s.t("名称"), Style: head},
			core.View{Style: core.Style{FlexGrow: 1}},
			headText(s.t("大小"), head, colSize, core.TextAlignEnd),
			colIf(func() bool { return showTypeCol(cx) }, headText(s.t("类型"), head, colType, 0)),
			colIf(func() bool { return showTimeCol(cx) }, headText(s.t("修改时间"), head, colTime, 0)),
			core.View{Style: core.Style{Width: core.Pt(colMenu), FlexShrink: 0}},
		},
	}
}

func (s *appState) fileRow(cx *core.Scope, o object) core.Node {
	colors := ui.ColorsOf(cx)
	kind := "file"
	if o.Dir {
		kind = "dir"
	} else {
		kind = objectKind(o)
	}
	size := humanSize(o.Size)
	if o.Dir {
		size = "—"
	}
	menu := core.View{
		Style: core.Style{Width: core.Pt(colMenu), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyEnd},
		Children: []core.Node{ui.Menu{
			Trigger: ui.Button{Icon: lucide.Ellipsis, Variant: ui.ButtonGhost, Size: ui.SizeSmall, Label: s.t("更多操作")},
			Children: []core.Node{
				ui.MenuItem{Text: s.t("打开"), Icon: lucide.FolderOpen, OnSelect: func() { s.activate(o) }},
				ui.MenuItem{Text: s.t("预览"), Icon: lucide.Eye, OnSelect: func() { s.previewObject(o) }, Disabled: func() bool { return o.Dir }},
				ui.MenuItem{Text: s.t("详情"), Icon: lucide.Info, OnSelect: func() { s.showDetailsFor(o) }, Disabled: func() bool { return o.Dir }},
				ui.MenuItem{Text: s.t("下载"), Icon: lucide.Download, OnSelect: func() { s.downloadObject(o) }, Disabled: func() bool { return o.Dir }},
				ui.MenuItem{Text: s.t("复制链接"), Icon: lucide.Link, OnSelect: func() { s.copyLink(o) }, Disabled: func() bool { return o.Dir }},
				ui.MenuItem{Text: s.t("在浏览器打开"), Icon: lucide.ExternalLink, OnSelect: func() { s.openInBrowser(o) }, Disabled: func() bool { return o.Dir }},
				ui.MenuItem{Text: s.t("重命名"), Icon: lucide.Pencil, OnSelect: func() { s.showRenameFor(o) }},
				ui.MenuItem{Text: s.t("复制到…"), Icon: lucide.Copy, OnSelect: func() { s.showCopyMoveFor(o, false) }},
				ui.MenuItem{Text: s.t("移动到…"), Icon: lucide.ArrowLeftRight, OnSelect: func() { s.showCopyMoveFor(o, true) }},
				ui.MenuItem{Text: s.t("删除"), Icon: lucide.Trash, Variant: ui.MenuItemDanger, OnSelect: func() { s.requestDelete([]string{o.Key}) }},
			},
		}},
	}
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: colGap, Width: core.Pct(100), BoxSizing: core.BorderBox, Padding: core.Edges{Left: listInset, Right: listInset}, Cursor: core.CursorPointer},
		StyleFunc: func() core.Style {
			c := ui.ColorsOf(cx)
			switch {
			case s.isSelected(o.Key):
				return core.Style{Background: ui.Alpha(c.Accent(), 0.16)}
			case s.cursor.Get() == o.Key:
				return core.Style{Background: ui.Alpha(c.Accent(), 0.10)}
			default:
				return core.Style{Hover: &core.Style{Background: ui.Alpha(c.Text(), 0.05)}}
			}
		},
		OnPress: func(e core.PressEvent) { s.rowPress(o, e) },
		Children: []core.Node{
			checkCell(ui.Checkbox{
				Checked:  func() bool { return s.isSelected(o.Key) },
				Label:    "选择 " + o.Name(),
				OnChange: func(bool) { s.toggleSelected(o.Key) },
			}),
			ui.IconOf(fileIcon(kind), colIcon, fileColor(kind)),
			core.Text{Content: o.Name(), MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
				Style: core.Style{Color: colors.Text(), FontSize: 13, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}},
			cellText(size, ui.TypeCaption, colSize, core.TextAlignEnd, colors.Text),
			colIf(func() bool { return showTypeCol(cx) }, cellText(o.Mime, ui.TypeCaption, colType, 0, colors.Muted)),
			colIf(func() bool { return showTimeCol(cx) }, cellText(formatPutTime(o.PutTime), ui.TypeCaption, colTime, 0, colors.Muted)),
			menu,
		},
	}
}

func (s *appState) activate(o object) {
	if o.Dir {
		s.openDir(o.Key)
		return
	}
	s.previewObject(o)
}

// rowPress highlights the row on a click (the checkbox is separate) and
// opens it on a double click; a modifier click checks it for batch work.
func (s *appState) rowPress(o object, e core.PressEvent) {
	if e.Clicks >= 2 {
		s.activate(o)
		return
	}
	if e.Modifiers.Control || e.Modifiers.Platform || e.Modifiers.Shift {
		s.toggleSelected(o.Key)
		return
	}
	s.cursor.Set(o.Key)
}

// searchRowPress highlights a result on a click and previews a file on a
// double click; a modifier click checks it for batch work.
func (s *appState) searchRowPress(o object, e core.PressEvent) {
	if e.Clicks >= 2 {
		if !o.Dir {
			s.previewObject(o)
		}
		return
	}
	if e.Modifiers.Control || e.Modifiers.Platform || e.Modifiers.Shift {
		s.search.toggleSelected(o.Key)
		return
	}
	s.search.cursor.Set(o.Key)
}

func (s *appState) allVisibleSelected() bool {
	objs := s.visible.Get()
	if len(objs) == 0 {
		return false
	}
	for _, o := range objs {
		if !s.isSelected(o.Key) {
			return false
		}
	}
	return true
}

func (s *appState) someVisibleSelected() bool {
	for _, o := range s.visible.Get() {
		if s.isSelected(o.Key) {
			return true
		}
	}
	return false
}

func (s *appState) selectAllVisible() {
	for _, o := range s.visible.Get() {
		if !s.isSelected(o.Key) {
			s.setSelected(o.Key, true)
		}
	}
}

func (s *appState) clearVisibleSelection() {
	for _, o := range s.visible.Get() {
		if s.isSelected(o.Key) {
			s.setSelected(o.Key, false)
		}
	}
}

func fileIcon(kind string) func() *core.SVG {
	switch kind {
	case "dir":
		return lucide.Folder
	case "image":
		return lucide.FileImage
	case "video":
		return lucide.FileVideoCamera
	case "audio":
		return lucide.FileMusic
	case "pdf":
		return lucide.FileText
	case "doc":
		return lucide.FileText
	case "markdown":
		return lucide.FileText
	case "text":
		return lucide.FileText
	case "code":
		return lucide.FileCode
	case "json":
		return lucide.FileBraces
	case "xml":
		return lucide.FileCode
	case "spreadsheet":
		return lucide.FileSpreadsheet
	case "presentation":
		return lucide.Presentation
	case "archive":
		return lucide.FileArchive
	case "font":
		return lucide.FileType
	case "app":
		return lucide.Package
	case "database":
		return lucide.Database
	case "key":
		return lucide.FileKey
	case "config":
		return lucide.FileCog
	case "log":
		return lucide.ScrollText
	case "script":
		return lucide.FileTerminal
	case "ebook":
		return lucide.BookOpen
	case "calendar":
		return lucide.Calendar
	case "contact":
		return lucide.FileUser
	default:
		return lucide.File
	}
}

// fileColor tints a file-type icon so each kind is recognizable at a
// glance, as file managers color their type icons.
func fileColor(kind string) func() core.Color {
	switch kind {
	case "dir":
		return solid(0x3B82F6) // blue
	case "image":
		return solid(0x16A34A) // green
	case "video":
		return solid(0x7C3AED) // violet
	case "audio":
		return solid(0xDB2777) // pink
	case "pdf":
		return solid(0xDC2626) // red
	case "doc":
		return solid(0x2563EB) // blue
	case "spreadsheet":
		return solid(0x15A34A) // green
	case "presentation":
		return solid(0xEA580C) // orange
	case "ebook":
		return solid(0x9333EA) // purple
	case "archive":
		return solid(0xD97706) // amber
	case "font":
		return solid(0x0D9488) // teal
	case "app":
		return solid(0x4F46E5) // indigo
	case "database":
		return solid(0x0891B2) // cyan
	case "key":
		return solid(0xB45309) // brown-amber
	case "config":
		return solid(0x64748B) // slate
	case "code":
		return solid(0x0284C7) // sky
	case "json":
		return solid(0xCA8A04) // yellow
	case "xml":
		return solid(0xEA580C) // orange
	case "markdown":
		return solid(0x4338CA) // indigo
	case "log":
		return solid(0x6B7280) // gray
	case "script":
		return solid(0x059669) // emerald
	case "calendar":
		return solid(0xE11D48) // rose
	case "contact":
		return solid(0x0EA5E9) // sky
	case "text":
		return solid(0x475569) // slate
	default:
		return solid(0x94A3B8) // gray
	}
}

func solid(hex uint32) func() core.Color {
	return func() core.Color { return core.RGB(hex) }
}

// --- advanced search -------------------------------------------------

// searchTypesLabel names the selected storage classes in the current
// language.
func (s *appState) searchTypesLabel() string {
	m := s.search.types.Get()
	names := make([]string, 0, len(m))
	for _, t := range storageTypeNames {
		if m[t.Value] {
			names = append(names, s.t(t.Name))
		}
	}
	if len(names) == 0 {
		return s.t("全部类型")
	}
	return strings.Join(names, "、")
}

func (s *appState) searchView(cx *core.Scope) core.Node {
	return core.View{
		Style:    core.Style{Direction: core.Column, FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
		Children: []core.Node{s.searchForm(cx), s.searchSelectionBar(cx), s.searchResults(cx)},
	}
}

// searchSelectionBar holds the file operations on the checked results of
// the advanced search.
func (s *appState) searchSelectionBar(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	compact := cx.Env().Window().Width < 640
	q := s.search
	none := func() bool { return len(q.activeKeys()) == 0 }
	single := func() bool { return len(q.activeKeys()) == 1 }
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Wrap: core.Wrap, Padding: core.Symmetric(6, 12),
			Background: ui.Alpha(colors.Accent(), 0.08), BorderWidth: 1, Borders: core.Edges{Bottom: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			ui.Checkbox{
				Checked:       func() bool { return q.allSelected() },
				Indeterminate: func() bool { return q.someSelected() && !q.allSelected() },
				Text:          s.t("全选"),
				OnChange: func(v bool) {
					if v {
						q.selectAll()
					} else {
						q.clearSelected()
					}
				},
			},
			s.selAction(compact, s.t("预览"), lucide.Eye, ui.ButtonGhost, false, func() bool { return !single() }, func() {
				if keys := q.activeKeys(); len(keys) == 1 {
					s.previewObject(object{Key: keys[0]})
				}
			}),
			s.selAction(compact, s.t("下载"), lucide.Download, ui.ButtonGhost, false, none, func() {
				s.requestDownload(q.activeKeys())
			}),
			s.selAction(compact, s.t("复制到"), lucide.Copy, ui.ButtonGhost, false, none, func() { s.showCopyMoveKeys(q.activeKeys(), false) }),
			s.selAction(compact, s.t("移动到"), lucide.ArrowLeftRight, ui.ButtonGhost, false, none, func() { s.showCopyMoveKeys(q.activeKeys(), true) }),
			s.selAction(compact, s.t("复制链接"), lucide.Link, ui.ButtonGhost, false, func() bool { return !single() }, func() {
				if keys := q.activeKeys(); len(keys) == 1 {
					s.copyLink(object{Key: keys[0]})
				}
			}),
			s.selAction(compact, s.t("删除"), lucide.Trash, ui.ButtonDanger, true, none, func() { s.requestDelete(q.activeKeys()) }),
			ui.IconButton{Icon: lucide.X, Label: s.t("清除选择"), Disabled: none, OnPress: func() { q.clearSelected(); q.cursor.Set("") }},
			core.View{Style: core.Style{FlexGrow: 1}},
			core.Text{ContentFunc: func() string {
				if n := q.selectedCount(); n > 0 {
					return s.tf("已勾选 %d 项", n)
				}
				if cur := q.cursor.Get(); cur != "" {
					return baseName(cur)
				}
				return s.t("未选择任何项")
			}, MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
				Style:     ui.TypeLabel.Merge(core.Style{FlexShrink: 1, MinWidth: core.Pt(0), TextAlign: core.TextAlignEnd, MaxWidth: core.Pt(360)}),
				StyleFunc: ui.Colored(colors.Text)},
		},
	}
}

func (s *appState) searchForm(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	q := s.search
	field := func(label string, in core.Node, width core.Points) core.Node {
		return core.View{Style: core.Style{Direction: core.Column, Gap: 4, Width: core.Pt(width)}, Children: []core.Node{
			core.Text{Content: label, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			in,
		}}
	}
	textInput := func(sig *core.Signal[string], placeholder string) core.Node {
		return ui.Input{Value: func() string { return sig.Get() }, OnChange: func(v string) { sig.Set(v) }, Placeholder: placeholder}
	}
	typeItems := []core.Node{ui.MenuItem{Text: s.t("全部类型（清除）"), OnSelect: func() { q.clearTypes() }}}
	for _, st := range storageTypeNames {
		st := st
		typeItems = append(typeItems, ui.MenuItem{
			Text:            s.tf("%s（%d）", s.t(st.Name), st.Value),
			NoCloseOnSelect: true,
			Checked:         func() bool { return q.hasType(st.Value) },
			OnSelect:        func() { q.toggleType(st.Value) },
		})
	}
	typeMenu := ui.Menu{
		Trigger: ui.Button{Label: s.t("存储类型")},
		Face: core.View{
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Width: core.Pct(100),
				Height: core.Pt(ui.SizeSmall.Height()), Padding: core.Symmetric(0, 10), BorderWidth: 1,
				BorderColor: colors.FieldBorder(), CornerRadius: ui.RadiusControl, Background: colors.Field(), OverflowX: core.Hidden},
			StyleFunc: func() core.Style { return core.Style{Hover: &core.Style{Background: colors.Hover()}} },
			Children: []core.Node{
				ui.IconOf(lucide.ListFilter, 14, colors.Muted),
				// A one-line, width-clipped label: the ellipsis shows only
				// when the text overflows its box.
				core.Text{ContentFunc: func() string { return s.searchTypesLabel() }, MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
					Style: core.Style{Color: colors.Text(), FontSize: 13, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}},
			},
		},
		MaxHeight: 260,
		Children:  typeItems,
	}
	fieldsRow := func(wrap bool) core.Node {
		st := core.Style{Direction: core.Row, Gap: 10}
		if wrap {
			st.Wrap = core.Wrap
		}
		return core.View{Style: st, Children: []core.Node{
			field(s.t("前缀 Prefix"), textInput(q.prefix, "images/2024/"), 110),
			field(s.t("后缀 Suffixes"), textInput(q.suffixes, "png,jpg,gif"), 110),
			field(s.t("MIME 类型"), textInput(q.mimes, "image/*,text/*"), 110),
			field(s.t("存储类型"), typeMenu, 220),
			field(s.t("最小大小"), textInput(q.minSize, "10K"), 120),
			field(s.t("最大大小"), textInput(q.maxSize, "5M"), 120),
			field(s.t("起始时间"), textInput(q.start, "2024-01-01 00:00:00"), 200),
			field(s.t("结束时间"), textInput(q.end, "2024-12-31 23:59:59"), 200),
		}}
	}
	return core.View{
		Style: core.Style{Direction: core.Column, Gap: 10, Padding: core.All(14),
			BorderWidth: 1, Borders: core.Edges{Bottom: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.Text{Content: s.t("高级搜索（对齐 qshell listbucket2）"), Style: ui.TypeTitle, StyleFunc: ui.Colored(colors.Text)},
				core.View{Style: core.Style{FlexGrow: 1}},
				ui.IconButton{Icon: func() *core.SVG {
					if q.collapsed.Get() {
						return lucide.ChevronDown()
					}
					return lucide.ChevronUp()
				}, Label: s.t("展开/折叠"), OnPress: func() { q.collapsed.Set(!q.collapsed.Get()) }},
			}},
			core.Show{When: func() bool { return !q.collapsed.Get() }, Then: func(cx *core.Scope) core.Node {
				return fieldsRow(true)
			}, Else: func(cx *core.Scope) core.Node {
				// Collapsed: one row of parameters, scrolled sideways on a
				// narrow screen instead of stacking into several rows.
				return core.Scroll{Horizontal: true, HideIndicator: true, Style: core.Style{Height: core.Pt(56), FlexShrink: 0},
					Children: []core.Node{fieldsRow(false)}}
			}},
			core.Show{When: func() bool { return !q.collapsed.Get() }, Then: func(cx *core.Scope) core.Node {
				return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16, Wrap: core.Wrap}, Children: []core.Node{
					ui.Checkbox{Text: s.t("包含目录"), Checked: func() bool { return q.includeDirs.Get() }, OnChange: func(v bool) { q.includeDirs.Set(v) }},
					ui.Checkbox{Text: s.t("可读大小"), Checked: func() bool { return q.readable.Get() }, OnChange: func(v bool) { q.readable.Set(v) }},
					ui.Checkbox{Text: s.t("导出含表头"), Checked: func() bool { return q.header.Get() }, OnChange: func(v bool) { q.header.Set(v) }},
					core.View{Style: core.Style{Width: core.Pt(160)}, Children: []core.Node{ui.Select[string]{
						Options:  []ui.Option[string]{{Value: "tab", Label: s.t("制表符分隔")}, {Value: "comma", Label: s.t("逗号分隔")}},
						Value:    func() string { return q.sep.Get() },
						OnChange: func(v string) { q.sep.Set(v) },
					}}},
				}}
			}, Else: func(*core.Scope) core.Node { return core.View{} }},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
				ui.Button{Text: s.t("开始搜索"), Icon: lucide.Search, Variant: ui.ButtonPrimary, OnPress: func() { s.runSearch() },
					Busy: func() bool { return q.scanning.Get() }, BusyLabel: s.t("扫描中")},
				ui.Button{Text: s.t("停止"), Icon: lucide.X, Variant: ui.ButtonOutline, OnPress: func() { s.runSearch() }, Disabled: func() bool { return !q.scanning.Get() }},
				ui.Button{Text: s.t("导出结果"), Icon: lucide.Save, Variant: ui.ButtonOutline, OnPress: func() { s.exportResults() }},
				core.Text{ContentFunc: func() string {
					return s.tf("已扫描 %d 项，匹配 %d 项", q.scanned.Get(), len(q.results.Get()))
				}, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			}},
			core.Show{When: func() bool { return q.errMsg.Get() != "" }, Then: func(cx *core.Scope) core.Node {
				return core.Text{ContentFunc: func() string { return q.errMsg.Get() }, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Danger)}
			}},
		},
	}
}

func (s *appState) searchResults(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	q := s.search
	return core.View{
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0), Padding: core.All(8)},
		Children: []core.Node{core.Show{
			When: func() bool { return len(q.results.Get()) == 0 },
			Then: func(cx *core.Scope) core.Node {
				return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, MinHeight: core.Pt(160)},
					Children: []core.Node{core.Text{ContentFunc: func() string {
						if q.scanning.Get() {
							return s.t("扫描中…")
						}
						return s.t("设置条件后点击“开始搜索”")
					}, Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)}}}
			},
			Else: func(cx *core.Scope) core.Node {
				return core.Scroll{
					Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
					Children: []core.Node{core.Each(
						func() []object { return q.results.Get() },
						func(o object) string { return o.Key },
						func(row *core.Scope, item func() object, _ func() int) core.Node {
							return s.searchRow(row, item())
						},
					)},
				}
			},
		}},
	}
}

func (s *appState) searchRow(row *core.Scope, o object) core.Node {
	colors := ui.ColorsOf(row)
	q := s.search
	size := displaySize(o.Size, q.readable.Get())
	kind := objectKind(o)
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: colGap, Width: core.Pct(100), BoxSizing: core.BorderBox, Padding: core.Edges{Left: listInset, Right: listInset}, Cursor: core.CursorPointer},
		StyleFunc: func() core.Style {
			c := ui.ColorsOf(row)
			switch {
			case q.isSelected(o.Key):
				return core.Style{Background: ui.Alpha(c.Accent(), 0.16)}
			case q.cursor.Get() == o.Key:
				return core.Style{Background: ui.Alpha(c.Accent(), 0.10)}
			default:
				return core.Style{Hover: &core.Style{Background: ui.Alpha(c.Text(), 0.05)}}
			}
		},
		OnPress: func(e core.PressEvent) { s.searchRowPress(o, e) },
		Children: []core.Node{
			checkCell(ui.Checkbox{
				Checked:  func() bool { return q.isSelected(o.Key) },
				Label:    "选择 " + o.Key,
				OnChange: func(bool) { q.toggleSelected(o.Key) },
			}),
			ui.IconOf(fileIcon(kind), colIcon, fileColor(kind)),
			core.Text{Content: o.Key, MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
				Style: core.Style{Color: colors.Text(), FontSize: 13, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}},
			cellText(size, ui.TypeCaption, colSize, core.TextAlignEnd, colors.Text),
			colIf(func() bool { return showTypeCol(row) }, cellText(o.Mime, ui.TypeCaption, colType, 0, colors.Muted)),
			colIf(func() bool { return showTimeCol(row) }, cellText(formatPutTime(o.PutTime), ui.TypeCaption, colTime, 0, colors.Muted)),
		},
	}
}

// uploadPanelView is the upload list: one row per file with its progress.
func (s *appState) uploadPanelView(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{
		Style: core.Style{Direction: core.Column, Height: core.Pt(200), BorderWidth: 1, Borders: core.Edges{Top: 1},
			BorderColor: colors.Border(), Background: colors.Field()},
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(4, 10)}, Children: []core.Node{
				core.Text{ContentFunc: func() string {
					return s.tf("上传列表（%d 个，进行中 %d）", len(s.uploads.Get()), s.activeUploads())
				}, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Text)},
				core.View{Style: core.Style{FlexGrow: 1}},
				ui.Button{Text: s.t("清除已完成"), Size: ui.SizeSmall, Variant: ui.ButtonGhost, OnPress: func() { s.clearFinishedUploads() }},
				ui.Button{Text: s.t("全部清除"), Size: ui.SizeSmall, Variant: ui.ButtonGhost, OnPress: func() { s.clearUploads() }},
				ui.IconButton{Icon: lucide.X, Label: s.t("收起"), OnPress: func() { s.uploadPanel.Set(false) }},
			}},
			core.Scroll{Style: core.Style{FlexGrow: 1, Padding: core.All(8)}, Children: []core.Node{core.Each(
				func() []uploadTask { return s.uploads.Get() },
				func(t uploadTask) int { return t.ID },
				func(row *core.Scope, item func() uploadTask, _ func() int) core.Node {
					return s.uploadRow(row, item)
				},
			)}},
		},
	}
}

func (s *appState) uploadRow(row *core.Scope, get func() uploadTask) core.Node {
	colors := ui.ColorsOf(row)
	stateColor := func() core.Color {
		switch get().State {
		case "完成":
			return colors.Up()
		case "失败":
			return colors.Danger()
		default:
			return colors.Muted()
		}
	}
	// percent and detail are separate fixed-width, right-aligned,
	// monospace cells, so the block sits at the right and neither the
	// percentage nor the sizes make it jump as they change.
	percent := func() string {
		t := get()
		if t.State != "上传中" {
			return ""
		}
		pct := 0
		if t.Total > 0 {
			pct = int(t.Done * 100 / t.Total)
		}
		return fmt.Sprintf("%3d%%", pct)
	}
	detail := func() string {
		t := get()
		switch t.State {
		case "完成":
			return "✓ " + s.t("完成")
		case "失败":
			if t.Error != "" {
				return "✗ " + s.t("失败") + "：" + t.Error
			}
			return "✗ " + s.t("失败")
		case "上传中":
			return fmt.Sprintf("%s / %s", humanSize(t.Done), humanSize(t.Total))
		default:
			return s.t("等待")
		}
	}
	cell := func(content func() string, width core.Points) core.Node {
		return core.Text{ContentFunc: content, MaxLines: 1,
			Style:     ui.TypeCaption.Merge(core.Style{Width: core.Pt(width), FlexShrink: 0, FontFamily: ui.Mono, TextAlign: core.TextAlignEnd}),
			StyleFunc: ui.Colored(stateColor)}
	}
	return core.View{
		Style: core.Style{Direction: core.Column, Gap: 4, Padding: core.Symmetric(4, 8)},
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				ui.IconOf(func() *core.SVG { return fileIcon(extKind(get().Name))() }, colIcon, func() core.Color { return fileColor(extKind(get().Name))() }),
				core.Text{ContentFunc: func() string { return get().Name }, MaxLines: 1,
					Style: core.Style{Color: colors.Text(), FontSize: 13, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}},
				cell(percent, 46),
				cell(detail, 176),
			}},
			core.Show{When: func() bool { st := get().State; return st == "上传中" || st == "等待" }, Then: func(cx *core.Scope) core.Node {
				return ui.Progress{Value: func() float32 {
					t := get()
					if t.Total <= 0 {
						return 0
					}
					return float32(t.Done) * 100 / float32(t.Total)
				}}
			}, Else: func(*core.Scope) core.Node { return core.View{} }},
		},
	}
}
