// Command qiniuoss is a Qiniu (七牛云) object-storage desktop
// manager built with NSGo.
package main

import (
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// narrow reports whether the window is too narrow to keep the sidebar
// beside the content.
func narrow(cx *core.Scope) func() bool {
	return func() bool { return cx.Env().Window().Width < 860 }
}

// Root is the app's first screen.
func Root(cx *core.Scope) core.Node {
	s := newState(cx)
	isNarrow := narrow(cx)
	// The whole view is built inside a Dynamic, so changing the language
	// (which every s.t read depends on) rebuilds the labels.
	return core.Dynamic(func(cx *core.Scope) core.Node {
		cx.SetTitle(s.t("七牛文件管理"))
		colors := ui.ColorsOf(cx)
		return core.View{
			Style: core.Style{Direction: core.Row, FlexGrow: 1, FlexShrink: 1, Background: colors.Background(),
				PaddingEnv: core.EnvAll(core.EnvSafeArea)},
			Children: []core.Node{
				s.sidebarSlot(cx, isNarrow),
				core.View{
					Style: core.Style{Direction: core.Column, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)},
					Children: []core.Node{
						s.header(cx, isNarrow),
						s.errorBanner(cx),
						core.View{
							Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
							Children: []core.Node{
								core.Show{
									When: func() bool { return s.tab.Get() == "search" },
									Then: s.searchView,
									Else: s.browseView,
								},
							},
						},
						core.Show{When: func() bool { return len(s.uploads.Get()) > 0 && s.uploadPanel.Get() }, Then: s.uploadPanelView, Else: func(*core.Scope) core.Node { return core.View{} }},
						s.statusBar(cx),
						core.Show{When: func() bool { return s.debugOn.Get() }, Then: s.debugPanel, Else: func(*core.Scope) core.Node { return core.View{} }},
					},
				},
				s.dialogs(cx),
			},
		}
	})
}

func (s *appState) sidebar(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{
		Style: core.Style{Direction: core.Column, Width: core.Pt(248), FlexShrink: 0, Gap: 10,
			Padding:    core.Edges{Top: 14, Right: 12, Bottom: 12, Left: 12},
			Background: colors.Surface(), BorderWidth: 1, Borders: core.Edges{Right: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				ui.IconOf(lucide.HardDrive, 18, colors.Accent),
				core.Text{Content: s.t("七牛文件管理"), Style: ui.TypeTitle, StyleFunc: ui.Colored(colors.Text)},
			}},
			core.Text{Content: s.t("账户"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			ui.Select[int]{
				OptionsFunc: func() []ui.Option[int] {
					list := s.accounts.Get()
					opts := make([]ui.Option[int], 0, len(list))
					for i, a := range list {
						name := a.Name
						if name == "" {
							name = s.tf("账户 %d", i+1)
						}
						opts = append(opts, ui.Option[int]{Value: i, Label: name})
					}
					return opts
				},
				Value:       func() int { return s.active.Get() },
				OnChange:    func(i int) { s.switchAccount(i) },
				Placeholder: s.t("选择账户"),
			},
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Button{Flex: 1, Text: s.t("账户"), Icon: lucide.Settings, Variant: ui.ButtonOutline, OnPress: func() { s.openAccountDialog() }},
				ui.Button{Flex: 1, Text: s.t("刷新"), Icon: lucide.RefreshCw, Variant: ui.ButtonOutline, OnPress: func() { s.refreshBuckets() }},
			}},
			core.View{Style: core.Style{Height: core.Pt(1), Background: colors.Border(), Margin: core.Edges{Top: 4, Bottom: 4}}},
			core.Text{Content: s.t("存储空间"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			s.bucketList(cx),
		},
	}
}

func (s *appState) bucketList(cx *core.Scope) core.Node {
	return core.View{
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
		Children: []core.Node{core.Scroll{
			Style: core.Style{FlexGrow: 1},
			Children: []core.Node{core.Each(
				func() []string { return s.buckets.Get() },
				func(b string) string { return b },
				func(row *core.Scope, item func() string, _ func() int) core.Node {
					return core.View{
						OnPress: func(core.PressEvent) { s.openBucket(item()) },
						Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
							Padding: core.Symmetric(7, 10), CornerRadius: ui.RadiusControl, Cursor: core.CursorPointer},
						StyleFunc: func() core.Style {
							c := ui.ColorsOf(row)
							if s.bucket.Get() == item() {
								return core.Style{Background: ui.Alpha(c.Accent(), 0.14), Color: c.Accent()}
							}
							return core.Style{Hover: &core.Style{Background: ui.Alpha(c.Text(), 0.06)}, Color: c.Text()}
						},
						Children: []core.Node{
							ui.IconOf(lucide.Folder, 16, fileColor("dir")),
							core.Text{Content: item(), Style: ui.TypeBody, MaxLines: 1, StyleFunc: ui.Colored(ui.ColorsOf(row).Text)},
						},
					}
				},
			)},
		}},
	}
}

func (s *appState) header(cx *core.Scope, isNarrow func() bool) core.Node {
	colors := ui.ColorsOf(cx)
	w := cx.Env().Window().Width
	compact := w < 640
	langItems := []core.Node{ui.MenuItem{Text: s.t("跟随系统"), Checked: func() bool { return s.langPref.Get() == "" },
		OnSelect: func() { s.setLangPref("") }}}
	for _, l := range appLanguages {
		l := l
		langItems = append(langItems, ui.MenuItem{Text: l.Name, Checked: func() bool { return s.langPref.Get() == l.Tag },
			OnSelect: func() { s.setLangPref(l.Tag) }})
	}
	pad := core.Symmetric(10, 16)
	if compact {
		pad = core.Symmetric(8, 10)
	}
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
			Padding: pad, BorderWidth: 1, Borders: core.Edges{Bottom: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			core.Show{When: isNarrow, Then: func(cx *core.Scope) core.Node {
				return ui.IconButton{Icon: lucide.List, Label: s.t("菜单"), OnPress: func() { s.showSidebar.Set(true) }}
			}, Else: func(cx *core.Scope) core.Node { return core.View{} }},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: []core.Node{
				ui.Tabs[string]{
					Variant: ui.TabsSegmented,
					Equal:   compact,
					Options: []ui.Option[string]{
						{Value: "browse", Label: s.t("浏览")},
						{Value: "search", Label: s.t("高级搜索")},
					},
					Value:    func() string { return s.tab.Get() },
					OnChange: func(v string) { s.tab.Set(v) },
				},
			}},
			ui.Menu{
				Trigger:   ui.Button{Icon: lucide.Languages, Variant: ui.ButtonGhost, Label: s.t("语言")},
				Children:  langItems,
				MaxHeight: 320,
			},
			ui.IconButton{Icon: lucide.Bug, Label: s.t("调试面板"), Selected: func() bool { return s.debugOn.Get() },
				OnPress: func() { s.debugOn.Set(!s.debugOn.Get()) }},
			core.Show{When: func() bool { return !compact }, Then: func(cx *core.Scope) core.Node {
				return core.Text{ContentFunc: func() string { return s.bucket.Get() }, Style: ui.TypeLabel, MaxLines: 1, StyleFunc: ui.Colored(colors.Muted)}
			}, Else: func(cx *core.Scope) core.Node { return core.View{} }},
		},
	}
}

// sidebarSlot shows the sidebar beside the content on a wide window, and
// as a dismissible overlay on a narrow one.
func (s *appState) sidebarSlot(cx *core.Scope, isNarrow func() bool) core.Node {
	return core.Show{
		When: func() bool { return !isNarrow() },
		Then: s.sidebar,
		Else: func(cx *core.Scope) core.Node {
			return core.Show{
				When: func() bool { return s.showSidebar.Get() },
				Then: func(cx *core.Scope) core.Node {
					colors := ui.ColorsOf(cx)
					return core.View{
						Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), ZIndex: 50},
						Children: []core.Node{
							core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0),
								Background: core.RGBA(0x000000, 0.4)}, OnPress: func(core.PressEvent) { s.showSidebar.Set(false) }},
							core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(260),
								Background: colors.Background(), PaddingEnv: core.EnvEdges{Top: core.EnvSafeArea, Bottom: core.EnvSafeArea}}, Children: []core.Node{s.sidebar(cx)}},
						},
					}
				},
				Else: func(cx *core.Scope) core.Node { return core.View{} },
			}
		},
	}
}

func (s *appState) errorBanner(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.Show{
		When: func() bool { return s.errMsg.Get() != "" },
		Then: func(cx *core.Scope) core.Node {
			return core.View{
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(8, 16),
					Background: ui.Alpha(colors.Danger(), 0.12)},
				Children: []core.Node{
					ui.IconOf(lucide.CircleAlert, 16, colors.Danger),
					core.Text{ContentFunc: func() string { return s.errMsg.Get() }, Style: ui.TypeBody.Merge(core.Style{FlexGrow: 1}), StyleFunc: ui.Colored(colors.Danger)},
					ui.IconButton{Icon: lucide.X, Label: s.t("关闭"), OnPress: func() { s.errMsg.Set("") }},
				},
			}
		},
	}
}

func (s *appState) statusBar(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(6, 16),
			BorderWidth: 1, Borders: core.Edges{Top: 1}, BorderColor: colors.Border()},
		Children: []core.Node{
			core.Show{When: func() bool { return s.loading.Get() }, Then: func(cx *core.Scope) core.Node {
				return ui.Spinner{Size: 14}
			}},
			core.Text{ContentFunc: func() string { return s.status.Get() }, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			core.View{Style: core.Style{FlexGrow: 1}},
			core.Text{ContentFunc: func() string {
				n := s.selectedCount()
				if n == 0 {
					return ""
				}
				return s.tf("已选 %d 项", n)
			}, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
		},
	}
}

// debugPanel shows the API calls the app makes and their raw responses.
func (s *appState) debugPanel(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	compact := cx.Env().Window().Width < 640
	copyLog := func() {
		var b strings.Builder
		for _, e := range s.debugLog.Get() {
			b.WriteString(e.Text)
			b.WriteByte('\n')
		}
		s.sc.SetClipboard(b.String())
		s.setStatus(s.t("调试日志已复制到剪贴板"))
	}
	buttons := []core.Node{
		ui.Button{Text: s.t("复制"), Size: ui.SizeSmall, Variant: ui.ButtonGhost, OnPress: copyLog},
		ui.Button{Text: s.t("清空"), Size: ui.SizeSmall, Variant: ui.ButtonGhost, OnPress: func() { s.clearDebug() }},
	}
	if compact {
		buttons = []core.Node{
			ui.IconButton{Icon: lucide.Copy, Label: s.t("复制"), OnPress: copyLog},
			ui.IconButton{Icon: lucide.Eraser, Label: s.t("清空"), OnPress: func() { s.clearDebug() }},
		}
	}
	header := append([]core.Node{
		core.Text{Content: s.t("调试 / API 调用"), MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, StyleFunc: ui.Colored(colors.Text)},
	}, buttons...)
	header = append(header, ui.IconButton{Icon: lucide.X, Label: s.t("关闭调试"), OnPress: func() { s.debugOn.Set(false) }})
	return core.View{
		Style: core.Style{Direction: core.Column, Height: core.Pt(220), BorderWidth: 1, Borders: core.Edges{Top: 1},
			BorderColor: colors.Border(), Background: colors.Field()},
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(4, 10)}, Children: header},
			core.Scroll{Style: core.Style{FlexGrow: 1, Padding: core.All(8)}, Children: []core.Node{core.Each(
				func() []debugEntry { return s.debugLog.Get() },
				func(e debugEntry) int { return e.Seq },
				func(row *core.Scope, item func() debugEntry, _ func() int) core.Node {
					return core.Text{ContentFunc: func() string { return item().Text }, Style: ui.TypeCode,
						StyleFunc: ui.Colored(ui.ColorsOf(row).Text), Selectable: core.SelectableOn}
				},
			)}},
		},
	}
}

func (s *appState) openAccountDialog() {
	list := s.accounts.Get()
	if len(list) > 0 {
		i := s.active.Get()
		if i < 0 || i >= len(list) {
			i = 0
		}
		s.loadAccountEditor(i)
	} else {
		s.editIdx.Set(-1)
		s.accName.Set("")
		s.accAK.Set("")
		s.accSK.Set("")
		s.accS3AK.Set("")
		s.accS3SK.Set("")
	}
	s.showAccounts.Set(true)
}

func (s *appState) loadAccountEditor(i int) {
	list := s.accounts.Get()
	if i < 0 || i >= len(list) {
		return
	}
	a := list[i]
	s.editIdx.Set(i)
	s.accName.Set(a.Name)
	s.accAK.Set(a.AK)
	s.accSK.Set(a.SK)
	s.accS3AK.Set(a.S3AccessKey)
	s.accS3SK.Set(a.S3SecretKey)
}

func (s *appState) saveAccountEditor() {
	name := trimSpace(s.accName.Get())
	ak := trimSpace(s.accAK.Get())
	sk := trimSpace(s.accSK.Get())
	if ak == "" || sk == "" {
		s.errMsg.Set(s.t("AccessKey 和 SecretKey 不能为空"))
		return
	}
	if name == "" {
		name = s.t("账户") + " " + itoa(len(s.accounts.Get())+1)
	}
	s3ak := trimSpace(s.accS3AK.Get())
	s3sk := trimSpace(s.accS3SK.Get())
	list := append([]account(nil), s.accounts.GetUntracked()...)
	i := s.editIdx.Get()
	if i >= 0 && i < len(list) {
		domains := list[i].Domains
		region, endpoint := list[i].S3Region, list[i].S3Endpoint
		list[i] = account{Name: name, AK: ak, SK: sk, Domains: domains,
			S3AccessKey: s3ak, S3SecretKey: s3sk, S3Region: region, S3Endpoint: endpoint}
	} else {
		list = append(list, account{Name: name, AK: ak, SK: sk, S3AccessKey: s3ak, S3SecretKey: s3sk})
		i = len(list) - 1
	}
	s.accounts.Set(list)
	if i >= 0 {
		s.active.Set(i)
	}
	s.persist()
	s.showAccounts.Set(false)
	s.client = nil
	s.s3cache = map[string]s3Endpoint{}
	s.bucket.Set("")
	s.objects.Set(nil)
	s.refreshBuckets()
}

func (s *appState) deleteAccount(i int) {
	list := s.accounts.Get()
	if i < 0 || i >= len(list) {
		return
	}
	list = append(list[:i:i], list[i+1:]...)
	s.accounts.Set(list)
	active := s.active.Get()
	if active >= len(list) {
		active = len(list) - 1
	}
	s.active.Set(active)
	s.persist()
	s.loadAccountEditor(active)
	if active < 0 {
		s.accName.Set("")
		s.accAK.Set("")
		s.accSK.Set("")
	}
	s.client = nil
	s.bucket.Set("")
	s.objects.Set(nil)
	s.refreshBuckets()
}

func (s *appState) importQshell() {
	a, err := importQshellAccount()
	if err != nil {
		s.errMsg.Set(err.Error())
		return
	}
	s.accName.Set(a.Name)
	s.accAK.Set(a.AK)
	s.accSK.Set(a.SK)
	s.setStatus(s.t("已从 qshell 读取账户信息，请保存"))
}

func trimSpace(s string) string { return strings.TrimSpace(s) }

func itoa(n int) string { return strconv.Itoa(n) }

func main() { nsgo.Run(Root, nsgo.WindowSize(1180, 780)) }
