package main

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

func (s *appState) dialogs(cx *core.Scope) core.Node {
	return core.View{Children: []core.Node{
		s.accountsDialog(cx),
		s.folderDialog(cx),
		s.renameDialog(cx),
		s.copyMoveDialog(cx),
		s.previewDialog(cx),
		s.detailsDialog(cx),
		s.domainsDialog(cx),
		s.deleteDialog(cx),
	}}
}

// bind turns a bool signal into an Open func and OnOpenChange pair.
func bind(sig *core.Signal[bool]) (func() bool, func(bool, base.Reason)) {
	return func() bool { return sig.Get() }, func(open bool, _ base.Reason) {
		if !open {
			sig.Set(false)
		}
	}
}

func footer(cancel func(), cancelText string, primary core.Node) []core.Node {
	nodes := []core.Node{ui.Button{Text: cancelText, Variant: ui.ButtonGhost, OnPress: cancel}}
	if primary != nil {
		nodes = append(nodes, primary)
	}
	return nodes
}

func (s *appState) accountsDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	open, onChange := bind(s.showAccounts)
	close := func() { s.showAccounts.Set(false) }
	field := func(label string, in core.Node) core.Node {
		return core.View{Style: core.Style{Direction: core.Column, Gap: 4}, Children: []core.Node{
			core.Text{Content: label, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)}, in,
		}}
	}
	return ui.Dialog{
		Open: open, OnOpenChange: onChange, Title: s.t("账户管理"), Description: s.t("配置七牛 AccessKey / SecretKey，密钥仅保存在本机。"),
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Column, Gap: 6}, Children: []core.Node{
				core.Text{Content: s.t("已有账户"), Style: ui.TypeLabel, StyleFunc: ui.Colored(colors.Text)},
				core.Each(
					func() []account { return s.accounts.Get() },
					func(a account) string { return a.Name + "|" + a.AK },
					func(row *core.Scope, item func() account, index func() int) core.Node {
						a := item()
						i := index()
						name := a.Name
						if name == "" {
							name = s.tf("账户 %d", i+1)
						}
						return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(6, 8),
							CornerRadius: ui.RadiusControl, BorderWidth: 1, BorderColor: ui.ColorsOf(row).Border()},
							Children: []core.Node{
								core.Text{Content: name, Style: ui.TypeBody.Merge(core.Style{FlexGrow: 1})},
								core.Text{Content: maskKey(a.AK), Style: ui.TypeCaption, StyleFunc: ui.Colored(ui.ColorsOf(row).Muted)},
								ui.Button{Text: s.t("使用"), Variant: ui.ButtonOutline, OnPress: func() { s.switchAccount(i); s.loadAccountEditor(i) }},
								ui.IconButton{Icon: lucide.Trash, Label: "删除", OnPress: func() { s.deleteAccount(i) }},
							}}
					},
				),
			}},
			core.View{Style: core.Style{Height: core.Pt(1), Background: colors.Border()}},
			field("名称", ui.Input{Value: func() string { return s.accName.Get() }, OnChange: func(v string) { s.accName.Set(v) }, Placeholder: s.t("例如：生产环境")}),
			field("AccessKey", ui.Input{Value: func() string { return s.accAK.Get() }, OnChange: func(v string) { s.accAK.Set(v) }, Placeholder: "AK"}),
			field("SecretKey", ui.Input{Value: func() string { return s.accSK.Get() }, OnChange: func(v string) { s.accSK.Set(v) }, Masked: true, Placeholder: "SK"}),
			ui.Collapsible{
				Open:         func() bool { return s.showS3.Get() },
				OnOpenChange: func(open bool) { s.showS3.Set(open) },
				Text:         s.t("S3 密钥（可选，用于没有绑定域名时下载/预览）"),
				Content: core.View{Style: core.Style{Direction: core.Column, Gap: 8, Margin: core.Edges{Top: 6}}, Children: []core.Node{
					core.Text{Content: s.t("留空时使用上面的 AccessKey / SecretKey 进行 S3 签名。"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
					field("S3 AccessKey", ui.Input{Value: func() string { return s.accS3AK.Get() }, OnChange: func(v string) { s.accS3AK.Set(v) }, Placeholder: s.t("控制台创建的 S3 AccessKey")}),
					field("S3 SecretKey", ui.Input{Value: func() string { return s.accS3SK.Get() }, OnChange: func(v string) { s.accS3SK.Set(v) }, Masked: true, Placeholder: "S3 SecretKey"}),
				}},
			},
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Button{Text: s.t("从 qshell 导入"), Icon: lucide.Download, Variant: ui.ButtonOutline, OnPress: func() { s.importQshell() }},
				ui.Button{Text: "新建", Icon: lucide.File, Variant: ui.ButtonGhost, OnPress: func() {
					s.editIdx.Set(-1)
					s.accName.Set("")
					s.accAK.Set("")
					s.accSK.Set("")
					s.accS3AK.Set("")
					s.accS3SK.Set("")
				}},
			}},
		},
		Footer: footer(close, s.t("关闭"), ui.Button{Text: s.t("保存"), Icon: lucide.Save, Variant: ui.ButtonPrimary, OnPress: func() { s.saveAccountEditor() }}),
	}
}

func maskKey(k string) string {
	if len(k) <= 8 {
		return k
	}
	return k[:4] + "…" + k[len(k)-4:]
}

func (s *appState) folderDialog(cx *core.Scope) core.Node {
	return s.simpleInputDialog(cx, s.showFolder, s.t("新建文件夹"), s.t("文件夹名称"), s.folderName, func() { s.makeFolder(s.folderName.Get()) })
}

func (s *appState) simpleInputDialog(cx *core.Scope, openSig *core.Signal[bool], title, label string, value *core.Signal[string], confirm func()) core.Node {
	colors := ui.ColorsOf(cx)
	open, onChange := bind(openSig)
	return ui.Dialog{
		Open: open, OnOpenChange: onChange, Title: title,
		Children: []core.Node{
			core.Text{Content: label, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			ui.Input{Value: func() string { return value.Get() }, OnChange: func(v string) { value.Set(v) }, OnSubmit: func(string) { confirm() }, AutoFocus: true},
		},
		Footer: footer(func() { openSig.Set(false) }, s.t("取消"), ui.Button{Text: s.t("确定"), Variant: ui.ButtonPrimary, OnPress: confirm}),
	}
}

func (s *appState) renameDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	open, onChange := bind(s.showRename)
	return ui.Dialog{
		Open: open, OnOpenChange: onChange, Title: s.t("重命名"),
		DescriptionFunc: func() string { return s.target.Get().Key },
		Children: []core.Node{
			core.Text{Content: s.t("新名称"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			ui.Input{Value: func() string { return s.renameValue.Get() }, OnChange: func(v string) { s.renameValue.Set(v) },
				OnSubmit: func(string) { s.applyRename() }, AutoFocus: true},
		},
		Footer: footer(func() { s.showRename.Set(false) }, s.t("取消"), ui.Button{Text: s.t("重命名"), Variant: ui.ButtonPrimary, OnPress: func() { s.applyRename() }}),
	}
}

func (s *appState) copyMoveDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	open, onChange := bind(s.showCopyMove)
	return ui.Dialog{
		Open: open, OnOpenChange: onChange,
		TitleFunc: func() string {
			if s.copyIsMove.Get() {
				return s.t("移动到")
			}
			return s.t("复制到")
		},
		DescriptionFunc: func() string {
			n := len(s.pendingTransfer)
			if n <= 1 {
				return s.target.Get().Key
			}
			return s.tf("共 %d 项", n)
		},
		Children: []core.Node{
			core.Text{Content: s.t("目标存储空间"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			ui.Select[string]{
				OptionsFunc: func() []ui.Option[string] {
					names := s.buckets.Get()
					opts := make([]ui.Option[string], 0, len(names))
					for _, n := range names {
						opts = append(opts, ui.Option[string]{Value: n, Label: n})
					}
					return opts
				},
				Value:    func() string { return s.copyBucket.Get() },
				OnChange: func(v string) { s.copyBucket.Set(v); s.loadDirTree(v) },
			},
			core.Text{Content: s.t("目标前缀（目录，可手动输入）"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			ui.Input{Value: func() string { return s.copyPrefix.Get() }, OnChange: func(v string) { s.copyPrefix.Set(v) }, Placeholder: "如 backup/2024/"},
			core.Text{Content: s.t("或从已有目录树中选择（点击「选择」）"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			ui.Tree{
				Roots:         s.treeRoots,
				Children:      s.treeChildren,
				Expanded:      func(key any) bool { p, _ := key.(string); return s.dirExpanded.Get()[p] },
				OnExpand:      s.treeOnExpand,
				Selection:     s.dirSel,
				EstimatedSize: 28,
				ToggleLabel:   s.t("展开"),
				Label:         s.t("目录树"),
				Style:         core.Style{Height: core.Pt(220), BorderWidth: 1, BorderColor: colors.Border(), CornerRadius: ui.RadiusControl, MinWidth: core.Pt(0)},
				Empty:         core.Text{Content: s.t("（没有子目录，可直接在下方输入前缀）"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
				Row: func(row *core.Scope, item ui.TreeItem) core.Node {
					key, _ := item.Key().(string)
					if key == dirLoadingKey {
						return core.Text{Content: "加载中…", Style: ui.TypeCaption, StyleFunc: ui.Colored(ui.ColorsOf(row).Muted)}
					}
					return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexGrow: 1, MinWidth: core.Pt(0)},
						Children: []core.Node{
							core.Text{Content: baseName(key) + "/", Style: ui.TypeBody.Merge(core.Style{FlexGrow: 1}), MaxLines: 1},
							ui.Button{Text: s.t("选择"), Size: ui.SizeSmall, Variant: ui.ButtonGhost,
								OnPress: func() { s.copyPrefix.Set(key) }},
						}}
				},
			},
		},
		Footer: footer(func() { s.showCopyMove.Set(false) }, s.t("取消"), ui.Button{
			TextFunc: func() string {
				if s.copyIsMove.Get() {
					return s.t("移动")
				}
				return s.t("复制")
			},
			Variant: ui.ButtonPrimary, OnPress: func() { s.applyCopyMove() }}),
	}
}

func (s *appState) previewDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	open := func() bool { return s.showPreview.Get() }
	onChange := func(o bool, _ base.Reason) {
		if !o {
			s.closePreview()
		}
	}
	return ui.Dialog{
		Open: open, OnOpenChange: onChange, Title: s.t("预览"),
		DescriptionFunc: func() string { return s.target.Get().Key },
		Children: []core.Node{
			core.Show{When: func() bool { return s.previewErr.Get() != "" }, Then: func(cx *core.Scope) core.Node {
				return core.Text{ContentFunc: func() string { return s.previewErr.Get() }, Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Danger)}
			}},
			core.Show{When: func() bool { return s.previewErr.Get() == "" && s.previewLoading.Get() }, Then: func(cx *core.Scope) core.Node {
				return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.All(16)}, Children: []core.Node{
					ui.Spinner{Size: 18},
					core.Text{Content: "加载中…", Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)},
				}}
			}},
			core.Show{When: func() bool { return s.previewErr.Get() == "" && !s.previewLoading.Get() }, Then: func(cx *core.Scope) core.Node {
				o := s.target.Get()
				local := s.previewPath.Get()
				url := s.previewURL.Get()
				switch objectKind(o) {
				case "image":
					src := url
					if local != "" {
						src = local
					}
					return core.Image{Source: core.Src(src), ResizeMode: core.Contain,
						Style: core.Style{Width: core.Pct(100), Height: core.Pt(360)}}
				case "video":
					src := url
					if local != "" {
						src = local
					}
					return core.Video{Source: src, Controls: true, Style: core.Style{Width: core.Pct(100), Height: core.Pt(360)}}
				case "audio":
					return core.Text{Content: s.t("音频文件请点击下方按钮在浏览器中打开。"), Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)}
				case "text":
					return core.Scroll{
						Style: core.Style{MaxHeight: core.Pt(360), Background: colors.Field(), CornerRadius: ui.RadiusControl, Padding: core.All(10)},
						Children: []core.Node{core.Text{ContentFunc: func() string { return s.previewText.Get() },
							Style: ui.TypeCode, Selectable: core.SelectableOn}},
					}
				case "pdf":
					return core.Text{Content: s.t("PDF 文件请点击下方按钮在浏览器中打开。"), Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)}
				default:
					return core.Text{Content: s.t("该类型暂不支持内嵌预览，可在浏览器中打开。"), Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)}
				}
			}},
		},
		Footer: []core.Node{
			ui.Button{Text: s.t("关闭"), Variant: ui.ButtonGhost, OnPress: func() { s.closePreview() }},
			ui.Button{Text: s.t("域名设置"), Icon: lucide.Globe, Variant: ui.ButtonOutline, OnPress: func() { s.closePreview(); s.openDomains() }},
			ui.Button{Text: s.t("在浏览器打开"), Icon: lucide.ExternalLink, Variant: ui.ButtonOutline, OnPress: func() {
				if u := s.previewURL.Get(); u != "" {
					s.sc.OpenURL(u, nil)
					return
				}
				s.openInBrowser(s.target.Get())
			}},
		},
	}
}

func (s *appState) detailsDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	open, onChange := bind(s.showDetails)
	row := func(k, v string) core.Node {
		return core.View{Style: core.Style{Direction: core.Row, Gap: 12, AlignItems: core.AlignStart}, Children: []core.Node{
			core.Text{Content: k, Style: ui.TypeLabel.Merge(core.Style{Width: core.Pt(96)}), StyleFunc: ui.Colored(colors.Muted)},
			core.Text{Content: v, Style: ui.TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), Selectable: core.SelectableOn},
		}}
	}
	return ui.Dialog{
		Open: open, OnOpenChange: onChange, Title: s.t("文件详情"),
		Children: []core.Node{core.View{Style: core.Style{Direction: core.Column, Gap: 8}, Children: []core.Node{
			row("Key", s.target.Get().Key),
			row("大小", fmt.Sprintf("%s (%d 字节)", humanSize(s.target.Get().Size), s.target.Get().Size)),
			row("MIME", s.target.Get().Mime),
			row("Hash", s.target.Get().Hash),
			row("存储类型", fmt.Sprintf("%d", s.target.Get().Type)),
			row(s.t("上传时间"), formatPutTime(s.target.Get().PutTime)),
			row("EndUser", s.target.Get().EndUser),
		}}},
		Footer: footer(func() { s.showDetails.Set(false) }, s.t("关闭"), nil),
	}
}

func (s *appState) domainsDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	open, onChange := bind(s.showDomains)
	return ui.Dialog{
		Open: open, OnOpenChange: onChange, Title: s.t("访问域名设置"),
		DescriptionFunc: func() string { return "存储空间：" + s.bucket.Get() },
		Children: []core.Node{
			core.Text{Content: s.t("用于生成私有/公开文件的访问链接，例如 cdn.example.com。"), Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
			ui.Input{Value: func() string { return s.domainInput.Get() }, OnChange: func(v string) { s.domainInput.Set(v) }, Placeholder: "https://cdn.example.com"},
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Button{Text: s.t("自动获取"), Icon: lucide.Download, Variant: ui.ButtonOutline, OnPress: func() { s.fetchDomains(s.bucket.Get()) }},
				ui.Button{Text: s.t("保存"), Icon: lucide.Save, Variant: ui.ButtonPrimary, OnPress: func() {
					s.saveDomain(s.bucket.Get(), s.domainInput.Get())
					s.setStatus(s.t("域名已保存"))
					s.showDomains.Set(false)
				}},
			}},
			core.Show{When: func() bool { return len(s.domainList.Get()) > 0 }, Then: func(cx *core.Scope) core.Node {
				return core.View{Style: core.Style{Direction: core.Column, Gap: 4}, Children: []core.Node{
					core.Text{Content: s.t("该空间绑定的域名"), Style: ui.TypeCaption, StyleFunc: ui.Colored(ui.ColorsOf(cx).Muted)},
					core.Each(
						func() []string { return s.domainList.Get() },
						func(d string) string { return d },
						func(row *core.Scope, item func() string, _ func() int) core.Node {
							return core.View{OnPress: func(core.PressEvent) { s.domainInput.Set(item()) },
								Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(4, 6),
									CornerRadius: ui.RadiusControl, Cursor: core.CursorPointer},
								StyleFunc: func() core.Style {
									return core.Style{Hover: &core.Style{Background: ui.Alpha(ui.ColorsOf(row).Text(), 0.06)}}
								},
								Children: []core.Node{
									ui.IconOf(lucide.Globe, 15, ui.ColorsOf(row).Muted),
									core.Text{Content: item(), Style: ui.TypeBody},
								}}
						},
					),
				}}
			}},
		},
		Footer: footer(func() { s.showDomains.Set(false) }, s.t("关闭"), nil),
	}
}

func (s *appState) deleteDialog(cx *core.Scope) core.Node {
	open, onChange := bind(s.showDelete)
	return ui.Dialog{
		Open: open, OnOpenChange: onChange, Title: s.t("确认删除"),
		DescriptionFunc: func() string {
			return fmt.Sprintf("将删除 %d 项（目录会连同其中所有对象一并删除），此操作不可撤销。", len(s.pendingDelete))
		},
		Role: core.RoleAlertDialog,
		Footer: footer(func() { s.pendingDelete = nil; s.showDelete.Set(false) }, s.t("取消"),
			ui.Button{Text: "删除", Icon: lucide.Trash, Variant: ui.ButtonDanger, OnPress: func() { s.confirmDelete() }}),
	}
}
