package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// searchState holds the advanced-search form and its results, mirroring
// the filters qshell listbucket2 offers.
type searchState struct {
	prefix      *core.Signal[string]
	suffixes    *core.Signal[string]
	mimes       *core.Signal[string]
	types       *core.Signal[map[int]bool]
	minSize     *core.Signal[string]
	maxSize     *core.Signal[string]
	start       *core.Signal[string]
	end         *core.Signal[string]
	includeDirs *core.Signal[bool]
	readable    *core.Signal[bool]
	header      *core.Signal[bool]
	sep         *core.Signal[string]

	results  *core.Signal[[]object]
	scanning *core.Signal[bool]
	scanned  *core.Signal[int]
	errMsg   *core.Signal[string]

	selected *core.Signal[map[string]bool]
	cursor   *core.Signal[string]

	cancel context.CancelFunc
}

func newSearchState(cx *core.Scope) *searchState {
	return &searchState{
		prefix:      core.NewSignal(cx, ""),
		suffixes:    core.NewSignal(cx, ""),
		mimes:       core.NewSignal(cx, ""),
		types:       core.NewSignalFunc(cx, map[int]bool{}, nil),
		minSize:     core.NewSignal(cx, ""),
		maxSize:     core.NewSignal(cx, ""),
		start:       core.NewSignal(cx, ""),
		end:         core.NewSignal(cx, ""),
		includeDirs: core.NewSignal(cx, false),
		readable:    core.NewSignal(cx, true),
		header:      core.NewSignal(cx, true),
		sep:         core.NewSignal(cx, "tab"),
		results:     core.NewSignalFunc(cx, []object(nil), nil),
		scanning:    core.NewSignal(cx, false),
		scanned:     core.NewSignal(cx, 0),
		errMsg:      core.NewSignal(cx, ""),
		selected:    core.NewSignalFunc(cx, map[string]bool{}, nil),
		cursor:      core.NewSignal(cx, ""),
	}
}

func (q *searchState) isSelected(key string) bool { return q.selected.Get()[key] }

func (q *searchState) toggleSelected(key string) {
	m := make(map[string]bool)
	for k := range q.selected.GetUntracked() {
		m[k] = true
	}
	if m[key] {
		delete(m, key)
	} else {
		m[key] = true
	}
	q.selected.Set(m)
}

func (q *searchState) selectedCount() int {
	n := 0
	for _, v := range q.selected.Get() {
		if v {
			n++
		}
	}
	return n
}

func (q *searchState) clearSelected() { q.selected.Set(map[string]bool{}) }

// Storage classes as Qiniu numbers them.
var storageTypeNames = []struct {
	Value int
	Name  string
}{
	{0, "标准"}, {1, "低频"}, {2, "归档"}, {3, "深度归档"}, {4, "归档直读"},
}

func storageTypeName(v int) string {
	for _, t := range storageTypeNames {
		if t.Value == v {
			return t.Name
		}
	}
	return itoa(v)
}

func (q *searchState) hasType(v int) bool { return q.types.Get()[v] }

func (q *searchState) toggleType(v int) {
	m := map[int]bool{}
	for k, on := range q.types.GetUntracked() {
		if on {
			m[k] = true
		}
	}
	if m[v] {
		delete(m, v)
	} else {
		m[v] = true
	}
	q.types.Set(m)
}

func (q *searchState) clearTypes() { q.types.Set(map[int]bool{}) }

func (q *searchState) typesLabel() string {
	m := q.types.Get()
	names := make([]string, 0, len(m))
	for _, t := range storageTypeNames {
		if m[t.Value] {
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		return "全部类型"
	}
	return strings.Join(names, "、")
}

// removeKeys drops keys from the results and the selection, after a
// delete or move took them away.
func (q *searchState) removeKeys(keys []string) {
	if len(keys) == 0 {
		return
	}
	gone := make(map[string]bool, len(keys))
	for _, k := range keys {
		gone[k] = true
	}
	removed := func(key string) bool {
		if gone[key] {
			return true
		}
		for g := range gone {
			if strings.HasSuffix(g, "/") && strings.HasPrefix(key, g) {
				return true
			}
		}
		return false
	}
	objs := q.results.GetUntracked()
	if len(objs) > 0 {
		out := make([]object, 0, len(objs))
		for _, o := range objs {
			if !removed(o.Key) {
				out = append(out, o)
			}
		}
		if len(out) != len(objs) {
			q.results.Set(out)
		}
	}
	m := make(map[string]bool)
	for k, v := range q.selected.GetUntracked() {
		if v && !removed(k) {
			m[k] = true
		}
	}
	q.selected.Set(m)
	if removed(q.cursor.GetUntracked()) {
		q.cursor.Set("")
	}
}

func (q *searchState) selectedKeys() []string {
	m := q.selected.Get()
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// activeKeys is the checked results, or the highlighted one when none is
// checked.
func (q *searchState) activeKeys() []string {
	if keys := q.selectedKeys(); len(keys) > 0 {
		return keys
	}
	if cur := q.cursor.Get(); cur != "" {
		return []string{cur}
	}
	return nil
}

func (q *searchState) allSelected() bool {
	objs := q.results.Get()
	if len(objs) == 0 {
		return false
	}
	for _, o := range objs {
		if !q.isSelected(o.Key) {
			return false
		}
	}
	return true
}

func (q *searchState) someSelected() bool {
	for _, o := range q.results.Get() {
		if q.isSelected(o.Key) {
			return true
		}
	}
	return false
}

func (q *searchState) selectAll() {
	for _, o := range q.results.Get() {
		if !q.isSelected(o.Key) {
			q.toggleSelected(o.Key)
		}
	}
}

func (q *searchState) filters() (searchFilters, error) {
	f := searchFilters{
		Prefix:      strings.TrimSpace(q.prefix.Get()),
		Suffixes:    splitList(q.suffixes.Get()),
		Mimes:       splitList(q.mimes.Get()),
		IncludeDirs: q.includeDirs.Get(),
	}
	var err error
	for t, on := range q.types.Get() {
		if on {
			f.Types = append(f.Types, t)
		}
	}
	sort.Ints(f.Types)
	if f.MinSize, err = parseSize(q.minSize.Get()); err != nil {
		return f, err
	}
	if f.MaxSize, err = parseSize(q.maxSize.Get()); err != nil {
		return f, err
	}
	if f.Start, err = parseTime(q.start.Get()); err != nil {
		return f, err
	}
	if f.End, err = parseTime(q.end.Get()); err != nil {
		return f, err
	}
	return f, nil
}

func (q *searchState) separator() string {
	if q.sep.Get() == "comma" {
		return ","
	}
	return "\t"
}

// appState is everything the app shows and does, held in signals so the
// view follows it.
type appState struct {
	sc *core.Scope
	st core.Storage

	accounts *core.Signal[[]account]
	active   *core.Signal[int]

	buckets *core.Signal[[]string]
	bucket  *core.Signal[string]

	prefix      *core.Signal[string]
	objects     *core.Signal[[]object]
	marker      *core.Signal[string]
	hasNext     *core.Signal[bool]
	loading     *core.Signal[bool]
	loadingMore *core.Signal[bool]
	status      *core.Signal[string]
	errMsg      *core.Signal[string]
	filter      *core.Signal[string]
	visible     *core.Memo[[]object]

	selected *core.Signal[map[string]bool]
	cursor   *core.Signal[string]

	showSidebar *core.Signal[bool]

	lang     *core.Signal[string]
	langPref *core.Signal[string]

	debugOn  *core.Signal[bool]
	debugLog *core.Signal[[]debugEntry]
	debugSeq int

	tab *core.Signal[string]

	showAccounts *core.Signal[bool]
	showFolder   *core.Signal[bool]
	showRename   *core.Signal[bool]
	showCopyMove *core.Signal[bool]
	showPreview  *core.Signal[bool]
	showDetails  *core.Signal[bool]
	showDomains  *core.Signal[bool]
	showS3       *core.Signal[bool]
	showDelete   *core.Signal[bool]

	pendingDelete   []string
	pendingTransfer []string

	editIdx *core.Signal[int]
	accName *core.Signal[string]
	accAK   *core.Signal[string]
	accSK   *core.Signal[string]
	accS3AK *core.Signal[string]
	accS3SK *core.Signal[string]
	s3cache map[string]s3Endpoint

	target      *core.Signal[object]
	renameValue *core.Signal[string]
	copyIsMove  *core.Signal[bool]
	copyBucket  *core.Signal[string]
	copyPrefix  *core.Signal[string]

	dirRoots    *core.Signal[[]string]
	dirCache    *core.Signal[map[string][]string]
	dirExpanded *core.Signal[map[string]bool]
	dirBucket   string
	dirSel      *ui.Selection

	domainInput *core.Signal[string]
	domainList  *core.Signal[[]string]

	previewText    *core.Signal[string]
	previewPath    *core.Signal[string]
	previewURL     *core.Signal[string]
	previewLoading *core.Signal[bool]
	previewErr     *core.Signal[string]
	previewTemp    string
	folderName     *core.Signal[string]

	uploads     *core.Signal[[]uploadTask]
	uploadPanel *core.Signal[bool]
	uploadSeq   int

	search *searchState

	client *client
}

func newState(cx *core.Scope) *appState {
	s := &appState{
		sc:             cx,
		st:             cx.Storage(),
		active:         core.NewSignal(cx, -1),
		buckets:        core.NewSignalFunc(cx, []string(nil), nil),
		bucket:         core.NewSignal(cx, ""),
		prefix:         core.NewSignal(cx, ""),
		objects:        core.NewSignalFunc(cx, []object(nil), nil),
		marker:         core.NewSignal(cx, ""),
		hasNext:        core.NewSignal(cx, false),
		loading:        core.NewSignal(cx, false),
		loadingMore:    core.NewSignal(cx, false),
		status:         core.NewSignal(cx, ""),
		errMsg:         core.NewSignal(cx, ""),
		filter:         core.NewSignal(cx, ""),
		selected:       core.NewSignalFunc(cx, map[string]bool{}, nil),
		cursor:         core.NewSignal(cx, ""),
		showSidebar:    core.NewSignal(cx, false),
		lang:           core.NewSignal(cx, normalizeLang(cx.Env().Language())),
		langPref:       core.NewSignal(cx, ""),
		debugOn:        core.NewSignal(cx, false),
		debugLog:       core.NewSignalFunc(cx, []debugEntry(nil), nil),
		tab:            core.NewSignal(cx, "browse"),
		showAccounts:   core.NewSignal(cx, false),
		showFolder:     core.NewSignal(cx, false),
		showRename:     core.NewSignal(cx, false),
		showCopyMove:   core.NewSignal(cx, false),
		showPreview:    core.NewSignal(cx, false),
		showDetails:    core.NewSignal(cx, false),
		showDomains:    core.NewSignal(cx, false),
		showS3:         core.NewSignal(cx, false),
		showDelete:     core.NewSignal(cx, false),
		editIdx:        core.NewSignal(cx, -1),
		accName:        core.NewSignal(cx, ""),
		accAK:          core.NewSignal(cx, ""),
		accSK:          core.NewSignal(cx, ""),
		accS3AK:        core.NewSignal(cx, ""),
		accS3SK:        core.NewSignal(cx, ""),
		s3cache:        map[string]s3Endpoint{},
		target:         core.NewSignal(cx, object{}),
		renameValue:    core.NewSignal(cx, ""),
		copyIsMove:     core.NewSignal(cx, false),
		copyBucket:     core.NewSignal(cx, ""),
		copyPrefix:     core.NewSignal(cx, ""),
		dirRoots:       core.NewSignalFunc(cx, []string(nil), nil),
		dirCache:       core.NewSignalFunc(cx, map[string][]string{}, nil),
		dirExpanded:    core.NewSignalFunc(cx, map[string]bool{}, nil),
		dirSel:         ui.NewSelection(cx, false),
		domainInput:    core.NewSignal(cx, ""),
		domainList:     core.NewSignalFunc(cx, []string(nil), nil),
		previewText:    core.NewSignal(cx, ""),
		previewPath:    core.NewSignal(cx, ""),
		previewURL:     core.NewSignal(cx, ""),
		previewLoading: core.NewSignal(cx, false),
		previewErr:     core.NewSignal(cx, ""),
		folderName:     core.NewSignal(cx, ""),
		uploads:        core.NewSignalFunc(cx, []uploadTask(nil), nil),
		uploadPanel:    core.NewSignal(cx, true),
		search:         newSearchState(cx),
	}
	s.loadStored()
	if pref, ok := s.st.GetItem("qiniu.lang"); ok && pref != "" {
		s.langPref.Set(pref)
		s.lang.Set(normalizeLang(pref))
	}
	s.visible = core.NewMemoFunc(cx, s.computeVisible, nil)
	s.sc.Post(func() { s.refreshBuckets() })
	return s
}

// setLangPref changes the shown language and remembers the choice. An
// empty pref follows the system language.
func (s *appState) setLangPref(pref string) {
	s.langPref.Set(pref)
	if pref == "" {
		s.lang.Set(normalizeLang(s.sc.Env().Language()))
	} else {
		s.lang.Set(normalizeLang(pref))
	}
	_ = s.st.SetItem("qiniu.lang", pref)
}

// debugEntry is one line in the debug panel.
type debugEntry struct {
	Seq  int
	Text string
}

// dbg appends a line to the debug panel, keeping the last 500.
func (s *appState) dbg(format string, args ...any) {
	if !s.debugOn.GetUntracked() {
		return
	}
	s.debugSeq++
	e := debugEntry{Seq: s.debugSeq, Text: fmt.Sprintf(format, args...)}
	s.debugLog.Update(func(cur []debugEntry) []debugEntry {
		cur = append(cur, e)
		if len(cur) > 500 {
			cur = cur[len(cur)-500:]
		}
		return cur
	})
}

func (s *appState) clearDebug() { s.debugLog.Set(nil) }

// debugList performs the same listing through the management API and logs
// the request URL and raw response, so what the app fetches on a
// directory switch is visible.
func (s *appState) debugList(bucket, prefix, delimiter, marker string, limit int) {
	if !s.debugOn.Get() {
		return
	}
	a, ok := s.current()
	if !ok {
		return
	}
	s.dbg("── 列表 bucket=%s prefix=%q delimiter=%q marker=%q limit=%d", bucket, prefix, delimiter, marker, limit)
	go func() {
		h, err := ucQueryHosts(context.Background(), a.AK, a.SK, bucket)
		if err != nil {
			s.sc.Post(func() { s.dbg("   uc 查询失败: %v", err) })
			return
		}
		reqURL, body, err := rawList(context.Background(), h.RsfHost, a.AK, a.SK, bucket, prefix, delimiter, marker, limit)
		s.sc.Post(func() {
			s.dbg("   → GET %s", reqURL)
			if err != nil {
				s.dbg("   ← ✗ %v  %s", err, truncate(body, 1000))
				return
			}
			s.dbg("   ← %s", truncate(body, 3000))
		})
	}()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(" + itoa(len(s)) + " bytes)"
}

func (s *appState) loadStored() {
	var list []account
	if raw, ok := s.st.GetItem("qiniu.accounts"); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &list)
	}
	active := -1
	if raw, ok := s.st.GetItem("qiniu.active"); ok && raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			active = n
		}
	}
	if active < 0 || active >= len(list) {
		active = -1
		if len(list) > 0 {
			active = 0
		}
	}
	s.accounts = core.NewSignalFunc(s.sc, list, nil)
	s.active.Set(active)
}

func (s *appState) persist() {
	if b, err := json.Marshal(s.accounts.GetUntracked()); err == nil {
		_ = s.st.SetItem("qiniu.accounts", string(b))
	}
	_ = s.st.SetItem("qiniu.active", strconv.Itoa(s.active.GetUntracked()))
}

func (s *appState) computeVisible() []object {
	objs := s.objects.Get()
	q := strings.TrimSpace(strings.ToLower(s.filter.Get()))
	if q == "" {
		return objs
	}
	out := make([]object, 0, len(objs))
	for _, o := range objs {
		if strings.Contains(strings.ToLower(o.Key), q) {
			out = append(out, o)
		}
	}
	return out
}

func (s *appState) setStatus(msg string) { s.status.Set(msg) }

func (s *appState) fail(err error) {
	if err == nil {
		return
	}
	s.errMsg.Set(err.Error())
	s.status.Set("")
}

func (s *appState) current() (account, bool) {
	list := s.accounts.Get()
	i := s.active.Get()
	if i < 0 || i >= len(list) {
		return account{}, false
	}
	return list[i], true
}

func (s *appState) ensureClient() (*client, account, bool) {
	a, ok := s.current()
	if !ok {
		s.errMsg.Set("请先添加并选择一个账户")
		return nil, account{}, false
	}
	if s.client == nil {
		s.client = newClient(a)
	}
	return s.client, a, true
}

func (s *appState) refreshBuckets() {
	a, ok := s.current()
	if !ok {
		s.buckets.Set(nil)
		s.setStatus(s.t("尚未配置账户，请点击左侧“管理账户”。"))
		return
	}
	s.client = newClient(a)
	c := s.client
	s.loading.Set(true)
	s.setStatus("正在获取存储空间…")
	go func() {
		names, err := c.buckets()
		s.sc.Post(func() {
			s.loading.Set(false)
			if err != nil {
				s.fail(err)
				return
			}
			s.buckets.Set(names)
			s.dbg("── 空间列表：%v", names)
			s.setStatus(fmt.Sprintf("账户 %s：%d 个存储空间", a.Name, len(names)))
			cur := s.bucket.GetUntracked()
			switch {
			case len(names) == 0:
				s.bucket.Set("")
				s.objects.Set(nil)
			case cur == "" || !contains(names, cur):
				s.openBucket(names[0])
			default:
				s.loadList(false)
			}
		})
	}()
}

func (s *appState) switchAccount(idx int) {
	s.active.Set(idx)
	s.client = nil
	s.s3cache = map[string]s3Endpoint{}
	s.bucket.Set("")
	s.objects.Set(nil)
	s.clearSelected()
	s.persist()
	s.refreshBuckets()
}

func (s *appState) openBucket(name string) {
	s.bucket.Set(name)
	s.prefix.Set("")
	s.filter.Set("")
	s.clearSelected()
	s.loadList(false)
}

func (s *appState) openDir(key string) {
	s.prefix.Set(key)
	s.filter.Set("")
	s.clearSelected()
	s.loadList(false)
}

func (s *appState) goUp() {
	p := s.prefix.Get()
	if p == "" {
		return
	}
	s.prefix.Set(parentPrefix(p))
	s.clearSelected()
	s.loadList(false)
}

func (s *appState) loadList(more bool) {
	c, _, ok := s.ensureClient()
	if !ok {
		return
	}
	bucket := s.bucket.Get()
	if bucket == "" {
		return
	}
	prefix := s.prefix.Get()
	marker := ""
	if more {
		if !s.hasNext.Get() || s.loadingMore.Get() {
			return
		}
		s.loadingMore.Set(true)
		marker = s.marker.Get()
	} else {
		// Clear the previous directory's rows at once, so entering an empty
		// directory shows nothing stale while it loads.
		s.objects.Set(nil)
		s.hasNext.Set(false)
		s.loading.Set(true)
		s.errMsg.Set("")
		s.debugList(bucket, prefix, "/", marker, 1000)
	}
	go func() {
		objs, prefixes, next, hasNext, err := c.list(context.Background(), bucket, prefix, "/", marker, 1000)
		s.sc.Post(func() {
			if err != nil {
				s.loading.Set(false)
				s.loadingMore.Set(false)
				s.fail(err)
				return
			}
			if more {
				s.objects.Update(func(cur []object) []object {
					return append(cur, objs...)
				})
				s.loadingMore.Set(false)
			} else {
				sortObjects(objs)
				s.objects.Set(objs)
				s.loading.Set(false)
			}
			names := make([]string, 0, len(objs))
			for _, o := range objs {
				names = append(names, o.Name())
			}
			vnames := make([]string, 0)
			for _, o := range s.visible.Get() {
				vnames = append(vnames, o.Name())
			}
			s.dbg("   解析: prefixes=%v keys=%v", prefixes, names)
			s.dbg("   界面列表(visible): %v", vnames)
			s.marker.Set(next)
			s.hasNext.Set(hasNext)
			s.setStatus(fmt.Sprintf("%s /%s：%d 项", bucket, prefix, len(objs)))
		})
	}()
}

func (s *appState) requestDelete(keys []string) {
	if len(keys) == 0 {
		return
	}
	s.pendingDelete = append([]string(nil), keys...)
	s.showDelete.Set(true)
}

func (s *appState) confirmDelete() {
	keys := s.pendingDelete
	s.pendingDelete = nil
	s.showDelete.Set(false)
	s.deleteKeys(keys)
}

func (s *appState) deleteKeys(keys []string) {
	c, _, ok := s.ensureClient()
	if !ok || len(keys) == 0 {
		return
	}
	bucket := s.bucket.Get()
	s.setStatus(fmt.Sprintf("正在删除 %d 项…", len(keys)))
	go func() {
		var all []string
		for _, k := range keys {
			if !strings.HasSuffix(k, "/") {
				all = append(all, k)
				continue
			}
			marker := ""
			for {
				objs, _, next, hasNext, err := c.list(context.Background(), bucket, k, "", marker, 1000)
				if err != nil {
					s.sc.Post(func() { s.fail(err) })
					return
				}
				for _, o := range objs {
					all = append(all, o.Key)
				}
				if !hasNext || next == "" || next == marker {
					break
				}
				marker = next
			}
			all = append(all, k)
		}
		err := c.remove(bucket, all)
		s.sc.Post(func() {
			if err != nil {
				s.fail(err)
				return
			}
			s.clearSelected()
			s.search.removeKeys(all)
			s.setStatus(fmt.Sprintf("已删除 %d 个对象", len(all)))
			s.loadList(false)
		})
	}()
}

func (s *appState) downloadObject(o object) {
	if o.Dir {
		return
	}
	url := s.openURLFor(o)
	if url == "" {
		s.errMsg.Set("未绑定域名且未配置 S3 密钥，无法下载")
		return
	}
	c := s.client
	name := o.Name()
	mime := o.Mime
	s.setStatus("正在下载 " + name)
	go func() {
		tmp, err := os.CreateTemp("", "qiniu-download-*")
		if err != nil {
			s.sc.Post(func() { s.fail(err) })
			return
		}
		path := tmp.Name()
		tmp.Close()
		if _, err := c.downloadToFile(context.Background(), url, path); err != nil {
			os.Remove(path)
			s.sc.Post(func() { s.fail(err) })
			return
		}
		s.sc.Post(func() {
			s.sc.SaveFile(core.SaveOptions{Name: name, MIME: mime, Path: path}, func(saved bool, dst string, err error) {
				os.Remove(path)
				if err != nil {
					s.fail(err)
					return
				}
				if saved {
					s.setStatus("已下载到 " + dst)
				}
			})
		})
	}()
}

func (s *appState) downloadSelected() { s.downloadKeys(s.activeKeys()) }

func (s *appState) downloadKeys(keys []string) {
	var files []string
	for _, key := range keys {
		if !strings.HasSuffix(key, "/") {
			files = append(files, key)
		}
	}
	if len(files) == 0 {
		s.errMsg.Set("请先选择要下载的文件")
		return
	}
	if len(files) == 1 {
		s.downloadObject(object{Key: files[0]})
		return
	}
	s.sc.PickDirectories(core.PickDirectoriesOptions{Title: "选择下载保存的目录"}, func(dirs []core.Directory, err error) {
		if err != nil {
			s.errMsg.Set(err.Error())
			return
		}
		if len(dirs) == 0 || dirs[0].Path == "" {
			s.downloadSequentially(files, "")
			return
		}
		s.downloadSequentially(files, dirs[0].Path)
	})
}

func (s *appState) downloadSequentially(keys []string, dir string) {
	c := s.client
	if c == nil {
		s.errMsg.Set("请先选择账户")
		return
	}
	bucket := s.bucket.Get()
	type item struct{ url, dst string }
	items := make([]item, 0, len(keys))
	for _, key := range keys {
		url := s.urlFor(bucket, key)
		if url == "" {
			s.errMsg.Set("未绑定域名且未配置 S3 密钥，无法下载")
			return
		}
		dst := filepath.Join(dir, baseName(key))
		if dir == "" {
			tmp, err := os.CreateTemp("", "qiniu-download-*")
			if err != nil {
				s.errMsg.Set(err.Error())
				return
			}
			dst = tmp.Name()
			tmp.Close()
		}
		items = append(items, item{url, dst})
	}
	s.setStatus(fmt.Sprintf("正在下载 %d 个文件…", len(items)))
	go func() {
		done := 0
		for _, it := range items {
			if _, err := c.downloadToFile(context.Background(), it.url, it.dst); err != nil {
				s.sc.Post(func() { s.fail(err) })
				return
			}
			done++
			n := done
			s.sc.Post(func() { s.setStatus(fmt.Sprintf("已下载 %d/%d", n, len(items))) })
		}
		s.sc.Post(func() {
			s.clearSelected()
			s.setStatus(fmt.Sprintf("已下载 %d 个文件", done))
		})
	}()
}

// selection state: the checkbox set (for batch operations) is kept apart
// from the cursor row that the pointer selected.

func (s *appState) isSelected(key string) bool { return s.selected.Get()[key] }

func (s *appState) selectedCount() int {
	n := 0
	for _, v := range s.selected.Get() {
		if v {
			n++
		}
	}
	return n
}

func (s *appState) setSelected(key string, on bool) {
	m := make(map[string]bool)
	for k := range s.selected.GetUntracked() {
		m[k] = true
	}
	if on {
		m[key] = true
	} else {
		delete(m, key)
	}
	s.selected.Set(m)
}

func (s *appState) toggleSelected(key string) { s.setSelected(key, !s.isSelected(key)) }

func (s *appState) clearSelected() { s.selected.Set(map[string]bool{}) }

func (s *appState) selectedKeys() []string {
	m := s.selected.Get()
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// activeKeys is what the batch actions operate on: the checked rows, or
// the cursor row when nothing is checked.
func (s *appState) activeKeys() []string {
	if keys := s.selectedKeys(); len(keys) > 0 {
		return keys
	}
	if cur := s.cursor.Get(); cur != "" {
		return []string{cur}
	}
	return nil
}

func (s *appState) applyRename() {
	c, _, ok := s.ensureClient()
	if !ok {
		return
	}
	o := s.target.Get()
	name := strings.TrimSpace(s.renameValue.Get())
	if name == "" {
		s.errMsg.Set("名称不能为空")
		return
	}
	bucket := s.bucket.Get()
	parent := parentPrefix(o.Key)
	s.showRename.Set(false)
	s.setStatus("正在重命名…")
	go func() {
		var err error
		if o.Dir {
			_, err = c.movePrefix(context.Background(), bucket, o.Key, parent+name+"/")
		} else {
			err = c.move(bucket, o.Key, bucket, parent+name)
		}
		s.sc.Post(func() {
			if err != nil {
				s.fail(err)
				return
			}
			s.setStatus("重命名完成")
			s.loadList(false)
		})
	}()
}

func (s *appState) applyCopyMove() {
	c, _, ok := s.ensureClient()
	if !ok {
		return
	}
	keys := s.pendingTransfer
	s.pendingTransfer = nil
	if len(keys) == 0 {
		s.showCopyMove.Set(false)
		return
	}
	dstBucket := s.copyBucket.Get()
	if dstBucket == "" {
		dstBucket = s.bucket.Get()
	}
	dstPrefix := normalizePrefix(s.copyPrefix.Get())
	srcBucket := s.bucket.Get()
	isMove := s.copyIsMove.Get()
	s.showCopyMove.Set(false)
	verb := "复制"
	if isMove {
		verb = "移动"
	}
	s.setStatus(fmt.Sprintf("正在%s %d 项…", verb, len(keys)))
	go func() {
		var err error
		for _, key := range keys {
			if strings.HasSuffix(key, "/") {
				_, err = c.transferPrefix(context.Background(), !isMove, srcBucket, dstBucket, key, dstPrefix+baseName(strings.TrimSuffix(key, "/"))+"/")
			} else {
				dstKey := dstPrefix + baseName(key)
				if isMove {
					err = c.move(srcBucket, key, dstBucket, dstKey)
				} else {
					err = c.copy(srcBucket, key, dstBucket, dstKey)
				}
			}
			if err != nil {
				break
			}
		}
		s.sc.Post(func() {
			if err != nil {
				s.fail(err)
				return
			}
			s.clearSelected()
			if isMove {
				s.search.removeKeys(keys)
			}
			s.setStatus(fmt.Sprintf("%s %d 项完成", verb, len(keys)))
			s.loadList(false)
		})
	}()
}

func (s *appState) makeFolder(name string) {
	c, _, ok := s.ensureClient()
	if !ok {
		return
	}
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" {
		s.errMsg.Set("文件夹名称不能为空")
		return
	}
	bucket := s.bucket.Get()
	key := s.prefix.Get() + name + "/"
	s.showFolder.Set(false)
	go func() {
		err := c.makeFolder(context.Background(), bucket, key)
		s.sc.Post(func() {
			if err != nil {
				s.fail(err)
				return
			}
			s.setStatus("已创建文件夹 " + name)
			s.loadList(false)
		})
	}()
}

func (s *appState) dropFiles(uris []string) {
	if s.bucket.Get() == "" {
		s.errMsg.Set("请先选择存储空间")
		return
	}
	var files []core.File
	for _, u := range uris {
		f, err := core.FileFromURL(u)
		if err != nil {
			s.errMsg.Set(err.Error())
			continue
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return
	}
	s.startUpload(files)
}

func (s *appState) beginUpload() {
	_, _, ok := s.ensureClient()
	if !ok {
		return
	}
	if s.bucket.Get() == "" {
		s.errMsg.Set("请先选择存储空间")
		return
	}
	s.sc.PickFiles(core.PickFilesOptions{Multiple: true, Title: "选择要上传的文件"}, func(files []core.File, err error) {
		if err != nil {
			s.errMsg.Set(err.Error())
			return
		}
		if len(files) == 0 {
			return
		}
		s.startUpload(files)
	})
}

// uploadTask is one file in the upload list.
type uploadTask struct {
	ID    int
	Name  string
	Key   string
	Total int64
	Done  int64
	State string // 等待 / 上传中 / 完成 / 失败
	Error string
}

func (s *appState) startUpload(files []core.File) {
	c, _, ok := s.ensureClient()
	if !ok {
		return
	}
	bucket := s.bucket.Get()
	prefix := s.prefix.Get()
	tasks := make([]uploadTask, 0, len(files))
	base := s.uploadSeq
	for _, f := range files {
		base++
		tasks = append(tasks, uploadTask{ID: base, Name: f.Name, Key: prefix + f.Name, Total: f.Size, State: "等待"})
	}
	s.uploadSeq = base
	s.uploads.Update(func(cur []uploadTask) []uploadTask { return append(cur, tasks...) })
	s.uploadPanel.Set(true)
	s.setStatus(fmt.Sprintf("已加入上传列表：%d 个文件", len(files)))

	go func() {
		for i, f := range files {
			id := tasks[i].ID
			key := tasks[i].Key
			file := f
			s.sc.Post(func() { s.setUpload(id, func(t *uploadTask) { t.State = "上传中" }) })
			last := -1
			// The SDK's OnProgress is func(fsize, uploaded): total first,
			// then the bytes uploaded so far.
			err := c.upload(context.Background(), bucket, key, file, func(fsize, uploaded int64) {
				if fsize <= 0 {
					return
				}
				pct := int(uploaded * 100 / fsize)
				if pct == last {
					return
				}
				last = pct
				s.sc.Post(func() { s.setUpload(id, func(t *uploadTask) { t.Done = uploaded; t.Total = fsize }) })
			})
			if err != nil {
				s.sc.Post(func() { s.setUpload(id, func(t *uploadTask) { t.State = "失败"; t.Error = err.Error() }) })
				continue
			}
			s.sc.Post(func() { s.setUpload(id, func(t *uploadTask) { t.State = "完成"; t.Done = t.Total }) })
		}
		s.sc.Post(func() {
			s.setStatus(fmt.Sprintf("上传完成：%d 个文件", len(files)))
			s.loadList(false)
		})
	}()
}

func (s *appState) setUpload(id int, fn func(*uploadTask)) {
	s.uploads.Update(func(cur []uploadTask) []uploadTask {
		out := append([]uploadTask(nil), cur...)
		for i := range out {
			if out[i].ID == id {
				fn(&out[i])
			}
		}
		return out
	})
}

func (s *appState) clearFinishedUploads() {
	s.uploads.Update(func(cur []uploadTask) []uploadTask {
		out := make([]uploadTask, 0, len(cur))
		for _, t := range cur {
			if t.State != "完成" {
				out = append(out, t)
			}
		}
		return out
	})
}

func (s *appState) clearUploads() { s.uploads.Set(nil) }

func (s *appState) activeUploads() int {
	n := 0
	for _, t := range s.uploads.Get() {
		if t.State == "等待" || t.State == "上传中" {
			n++
		}
	}
	return n
}

func (s *appState) domainFor(bucket string) string {
	a, ok := s.current()
	if !ok || a.Domains == nil {
		return ""
	}
	return normalizeDomain(a.Domains[bucket])
}

func (s *appState) saveDomain(bucket, domain string) {
	list := append([]account(nil), s.accounts.GetUntracked()...)
	i := s.active.Get()
	if i < 0 || i >= len(list) {
		return
	}
	if list[i].Domains == nil {
		list[i].Domains = map[string]string{}
	}
	domain = normalizeDomain(domain)
	if domain == "" {
		delete(list[i].Domains, bucket)
	} else {
		list[i].Domains[bucket] = domain
	}
	s.accounts.Set(list)
	s.persist()
}

func (s *appState) fetchDomains(bucket string) {
	c, _, ok := s.ensureClient()
	if !ok {
		return
	}
	s.domainList.Set(nil)
	s.setStatus("正在获取绑定域名…")
	go func() {
		domains, err := c.domains(bucket)
		s.sc.Post(func() {
			if err != nil {
				s.fail(err)
				return
			}
			s.domainList.Set(domains)
			s.setStatus(fmt.Sprintf("域名：%d 个", len(domains)))
		})
	}()
}

func (s *appState) runSearch() {
	c, _, ok := s.ensureClient()
	if !ok {
		return
	}
	bucket := s.bucket.Get()
	if bucket == "" {
		s.search.errMsg.Set("请先选择存储空间")
		return
	}
	f, err := s.search.filters()
	if err != nil {
		s.search.errMsg.Set(err.Error())
		return
	}
	if s.search.scanning.Get() {
		if s.search.cancel != nil {
			s.search.cancel()
		}
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.search.cancel = cancel
	s.search.scanning.Set(true)
	s.search.errMsg.Set("")
	s.search.scanned.Set(0)
	s.search.results.Set(nil)
	debug := s.debugOn.Get()
	if debug {
		s.dbg("══ 开始搜索 bucket=%s prefix=%q suffixes=%v mimes=%v types=%v min=%d max=%d start=%v end=%v",
			bucket, f.Prefix, f.Suffixes, f.Mimes, f.Types, f.MinSize, f.MaxSize, f.Start, f.End)
	}

	go func() {
		marker := ""
		var results []object
		count := 0
		for {
			if ctx.Err() != nil {
				break
			}
			pageMarker := marker
			objs, prefixes, next, hasNext, err := c.list(ctx, bucket, f.Prefix, "", marker, 1000)
			if err != nil {
				if ctx.Err() != nil {
					break
				}
				s.sc.Post(func() {
					s.search.scanning.Set(false)
					s.search.errMsg.Set(err.Error())
					s.dbg("   ✗ 搜索请求失败: %v", err)
				})
				return
			}
			for _, o := range objs {
				count++
				if f.match(o) {
					results = append(results, o)
				}
			}
			snapshot := append([]object(nil), results...)
			total := count
			s.sc.Post(func() {
				s.search.results.Set(snapshot)
				s.search.scanned.Set(total)
				s.dbg("   页 list prefix=%q marker=%q → %d 项（prefixes=%d），累计 %d，匹配 %d",
					f.Prefix, pageMarker, len(objs), len(prefixes), total, len(snapshot))
			})
			if debug {
				s.sc.Post(func() { s.debugList(bucket, f.Prefix, "", pageMarker, 1000) })
			}
			if !hasNext || next == "" || next == marker || count >= 200000 {
				break
			}
			marker = next
		}
		s.sc.Post(func() {
			s.search.scanning.Set(false)
			sortObjects(results)
			s.search.results.Set(results)
			s.search.scanned.Set(count)
			s.dbg("══ 搜索结束：累计 %d，匹配 %d", count, len(results))
		})
	}()
}

func (s *appState) exportResults() {
	q := s.search
	results := q.results.Get()
	if len(results) == 0 {
		q.errMsg.Set("没有可导出的结果")
		return
	}
	text := buildListText(results, q.separator(), q.readable.Get(), q.header.Get())
	name := "qiniu-list-" + s.bucket.Get() + ".tsv"
	if q.separator() == "," {
		name = "qiniu-list-" + s.bucket.Get() + ".csv"
	}
	s.sc.SaveFile(core.SaveOptions{Name: name, Data: []byte(text), MIME: "text/plain"}, func(saved bool, path string, err error) {
		if err != nil {
			q.errMsg.Set(err.Error())
			return
		}
		if saved {
			s.setStatus("已导出到 " + path)
		}
	})
}

func (s *appState) openURLFor(o object) string {
	return s.urlFor(s.bucket.Get(), o.Key)
}

// urlFor returns a URL to read key from bucket: a signed CDN link when a
// domain is bound, else an S3 presigned link when S3 keys are set.
func (s *appState) urlFor(bucket, key string) string {
	if bucket == "" || key == "" || strings.HasSuffix(key, "/") {
		return ""
	}
	if domain := s.domainFor(bucket); domain != "" && s.client != nil {
		return s.client.privateURL(domain, key, 0)
	}
	return s.s3Presign(bucket, key)
}

// s3EndpointFor returns the S3 endpoint for bucket, from the account's
// override or discovered from Qiniu once and cached.
func (s *appState) s3EndpointFor(a account, bucket string) (s3Endpoint, bool) {
	if a.S3Endpoint != "" {
		ep := s3Endpoint{Host: a.S3Endpoint, Region: a.S3Region}
		if ep.Region == "" {
			ep.Region = regionFromS3Host(ep.Host)
		}
		return ep, true
	}
	if ep, ok := s.s3cache[bucket]; ok {
		return ep, true
	}
	ep, err := discoverS3(context.Background(), a.AK, a.SK, bucket)
	if err != nil {
		return s3Endpoint{}, false
	}
	s.s3cache[bucket] = ep
	return ep, true
}

// s3Presign returns a presigned S3 GET URL for key when the account has
// S3 keys, else "".
// previewURLFor returns the URL the preview reads from: always the S3
// presigned link when S3 keys are set, else the CDN link.
func (s *appState) previewURLFor(o object) string {
	if o.Dir || o.Key == "" {
		return ""
	}
	if u := s.s3Presign(s.bucket.Get(), o.Key); u != "" {
		return u
	}
	return s.openURLFor(o)
}

func (s *appState) s3Presign(bucket, key string) string {
	a, ok := s.current()
	if !ok {
		return ""
	}
	// Fall back to the account's AccessKey / SecretKey when no S3 keys
	// are set.
	ak, sk := a.S3AccessKey, a.S3SecretKey
	if ak == "" || sk == "" {
		ak, sk = a.AK, a.SK
	}
	if ak == "" || sk == "" {
		return ""
	}
	ep, ok := s.s3EndpointFor(a, bucket)
	if !ok || ep.Host == "" || ep.Region == "" {
		return ""
	}
	return presignGET(ep, ak, sk, bucket, key, time.Hour)
}

// previewLocalLimit is the size under which a preview downloads the file
// to a local temp file first.
const previewLocalLimit = 10 << 20

func (s *appState) previewObject(o object) {
	if o.Dir {
		return
	}
	s.target.Set(o)
	s.showPreview.Set(true)
	s.preparePreview(o)
}

func (s *appState) preparePreview(o object) {
	s.clearPreviewTemp()
	s.previewText.Set("")
	s.previewPath.Set("")
	s.previewURL.Set("")
	s.previewErr.Set("")
	url := s.previewURLFor(o)
	if url == "" {
		s.previewErr.Set("无法获取预览文件：请配置 S3 密钥（推荐），或为该空间绑定访问域名。")
		return
	}
	s.previewURL.Set(url)
	// Size and MIME may be unknown for an object built from a search
	// result: stat it first.
	if o.Size <= 0 || o.Mime == "" {
		c := s.client
		bucket := s.bucket.Get()
		s.previewLoading.Set(true)
		go func() {
			mo := o
			if fi, err := c.stat(bucket, o.Key); err == nil {
				mo = fi
			}
			s.sc.Post(func() {
				s.target.Set(mo)
				s.finishPreview(mo, url)
			})
		}()
		return
	}
	s.finishPreview(o, url)
}

func (s *appState) finishPreview(o object, url string) {
	kind := objectKind(o)
	if o.Size >= previewLocalLimit || !isInlineKind(kind) {
		// Large or non-inline: stream from the remote URL; text is fetched.
		s.previewLoading.Set(false)
		if isTextKind(kind) {
			s.previewText.Set("加载中…")
			s.loadPreviewText(url)
		}
		return
	}
	c := s.client
	s.previewLoading.Set(true)
	go func() {
		tmp, err := os.CreateTemp("", "qiniu-preview-*")
		if err != nil {
			s.sc.Post(func() { s.previewLoading.Set(false); s.previewErr.Set(err.Error()) })
			return
		}
		path := tmp.Name()
		tmp.Close()
		if _, err := c.downloadToFile(context.Background(), url, path); err != nil {
			os.Remove(path)
			s.sc.Post(func() { s.previewLoading.Set(false); s.previewErr.Set(err.Error()) })
			return
		}
		s.sc.Post(func() {
			s.previewLoading.Set(false)
			switch {
			case kind == "image" || kind == "video":
				s.previewTemp = path
				s.previewPath.Set(path)
			case isTextKind(kind):
				b, err := os.ReadFile(path)
				os.Remove(path)
				if err != nil {
					s.previewErr.Set(err.Error())
					return
				}
				s.previewText.Set(string(b))
			}
		})
	}()
}

// clearPreviewTemp removes the temp file a small-file preview downloaded.
func (s *appState) clearPreviewTemp() {
	if s.previewTemp != "" {
		os.Remove(s.previewTemp)
		s.previewTemp = ""
	}
}

func (s *appState) closePreview() {
	s.showPreview.Set(false)
	s.clearPreviewTemp()
	s.previewPath.Set("")
	s.previewURL.Set("")
	s.previewText.Set("")
	s.previewErr.Set("")
}

func (s *appState) loadPreviewText(url string) {
	c := s.client
	if c == nil {
		return
	}
	go func() {
		text, err := c.fetchText(context.Background(), url, 256*1024)
		s.sc.Post(func() {
			if err != nil {
				s.previewText.Set("加载失败：" + err.Error())
				return
			}
			s.previewText.Set(text)
		})
	}()
}

func (s *appState) showDetailsFor(o object) {
	s.target.Set(o)
	s.showDetails.Set(true)
}

func (s *appState) showRenameFor(o object) {
	s.target.Set(o)
	s.renameValue.Set(o.Name())
	s.showRename.Set(true)
}

func (s *appState) showCopyMoveFor(o object, move bool) {
	s.target.Set(o)
	s.pendingTransfer = []string{o.Key}
	s.copyIsMove.Set(move)
	s.copyBucket.Set(s.bucket.Get())
	s.copyPrefix.Set(s.prefix.Get())
	s.showCopyMove.Set(true)
	s.loadDirTree(s.bucket.Get())
}

func (s *appState) showCopyMoveSelected(move bool) { s.showCopyMoveKeys(s.activeKeys(), move) }

func (s *appState) showCopyMoveKeys(keys []string, move bool) {
	if len(keys) == 0 {
		return
	}
	if len(keys) == 1 {
		s.showCopyMoveFor(object{Key: keys[0], Dir: strings.HasSuffix(keys[0], "/")}, move)
		s.pendingTransfer = keys
		return
	}
	s.pendingTransfer = keys
	s.copyIsMove.Set(move)
	s.copyBucket.Set(s.bucket.Get())
	s.copyPrefix.Set(s.prefix.Get())
	s.showCopyMove.Set(true)
	s.loadDirTree(s.bucket.Get())
}

// dirLoadingKey is a placeholder row built for a directory whose children
// are still being listed.
const dirLoadingKey = "\x00loading"

// loadDirTree rebuilds the destination directory tree for bucket.
func (s *appState) loadDirTree(bucket string) {
	s.dirBucket = bucket
	s.dirExpanded.Set(map[string]bool{})
	s.dirCache.Set(map[string][]string{})
	s.dirRoots.Set(nil)
	s.loadDirs(bucket, "")
}

// loadDirs lists the sub-directories of prefix and caches them.
func (s *appState) loadDirs(bucket, prefix string) {
	c, _, ok := s.ensureClient()
	if !ok || bucket == "" {
		return
	}
	s.dbg("── 目录树 bucket=%s prefix=%q", bucket, prefix)
	go func() {
		_, raw, _, _, err := c.list(context.Background(), bucket, prefix, "/", "", 1000)
		// Keep only real child directories: drop the prefix's own marker
		// and anything not under prefix.
		prefixes := childDirs(prefix, raw)
		s.sc.Post(func() {
			if err != nil {
				s.errMsg.Set(err.Error())
				return
			}
			s.dbg("   原始前缀=%v  子目录=%v", raw, prefixes)
			if prefix == "" {
				s.dirRoots.Set(prefixes)
				return
			}
			m := map[string][]string{}
			for k, v := range s.dirCache.GetUntracked() {
				m[k] = v
			}
			m[prefix] = prefixes
			s.dirCache.Set(m)
		})
	}()
}

func (s *appState) treeSetExpanded(key string, on bool) {
	m := map[string]bool{}
	for k, v := range s.dirExpanded.GetUntracked() {
		m[k] = v
	}
	if on {
		m[key] = true
	} else {
		delete(m, key)
	}
	s.dirExpanded.Set(m)
}

func (s *appState) treeRoots() []any {
	out := make([]any, 0, len(s.dirRoots.Get()))
	for _, p := range s.dirRoots.Get() {
		out = append(out, p)
	}
	return out
}

func (s *appState) treeChildren(key any) []any {
	p, _ := key.(string)
	if p == "" || p == dirLoadingKey {
		return nil
	}
	if s.dirExpanded.Get()[p] {
		if kids, ok := s.dirCache.Get()[p]; ok {
			out := make([]any, 0, len(kids))
			for _, k := range kids {
				out = append(out, k)
			}
			return out
		}
		return []any{dirLoadingKey}
	}
	// Collapsed: report a branch (so the chevron shows) unless we already
	// know it has no sub-directories.
	if kids, ok := s.dirCache.Get()[p]; ok && len(kids) == 0 {
		return nil
	}
	return []any{p}
}

func (s *appState) treeOnExpand(key any, expanded bool) {
	p, _ := key.(string)
	if p == "" || p == dirLoadingKey {
		return
	}
	s.treeSetExpanded(p, expanded)
	if expanded {
		if _, ok := s.dirCache.Get()[p]; !ok {
			s.loadDirs(s.dirBucket, p)
		}
	}
}

func (s *appState) copyLink(o object) {
	url := s.openURLFor(o)
	if url == "" {
		s.errMsg.Set("未绑定域名且未配置 S3 密钥，无法生成链接")
		return
	}
	s.sc.SetClipboard(url)
	s.setStatus("已复制链接")
}

func (s *appState) openInBrowser(o object) {
	url := s.openURLFor(o)
	if url == "" {
		s.errMsg.Set("未绑定域名且未配置 S3 密钥，无法打开")
		return
	}
	s.sc.OpenURL(url, nil)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
