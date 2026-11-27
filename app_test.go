package main

import (
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/qiniu/go-sdk/v7/storage"
)

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		0:          "0 B",
		512:        "512 B",
		1024:       "1.00 KB",
		1536:       "1.50 KB",
		1048576:    "1.00 MB",
		1073741824: "1.00 GB",
	}
	for in, want := range cases {
		if got := humanSize(in); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"":     0,
		"1024": 1024,
		"10K":  10 * 1024,
		"5M":   5 * 1024 * 1024,
		"2g":   2 * 1024 * 1024 * 1024,
		"1.5M": 1572864,
	}
	for in, want := range cases {
		got, err := parseSize(in)
		if err != nil {
			t.Fatalf("parseSize(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("parseSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseTime(t *testing.T) {
	got, err := parseTime("2024-03-01-12-30-45")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 3, 1, 12, 30, 45, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("parseTime = %v, want %v", got, want)
	}
	if empty, err := parseTime("  "); err != nil || !empty.IsZero() {
		t.Errorf("parseTime(empty) = %v, %v", empty, err)
	}
	if _, err := parseTime("not-a-date"); err == nil {
		t.Error("parseTime(bad) expected error")
	}
}

func TestMatchMime(t *testing.T) {
	cases := []struct {
		pattern, mime string
		want          bool
	}{
		{"*", "image/png", true},
		{"*/*", "image/png", true},
		{"image/*", "image/png", true},
		{"image/*", "text/plain", false},
		{"image/png", "image/png", true},
		{"png", "image/png", true},
	}
	for _, c := range cases {
		if got := matchMime(c.pattern, c.mime); got != c.want {
			t.Errorf("matchMime(%q,%q) = %v, want %v", c.pattern, c.mime, got, c.want)
		}
	}
}

func TestSearchFilters(t *testing.T) {
	now := time.Now()
	put := func(tm time.Time) int64 { return tm.Unix() * 10_000_000 }
	objs := []object{
		{Key: "images/a.png", Size: 100, Mime: "image/png", PutTime: put(now.Add(-2 * time.Hour))},
		{Key: "images/b.jpg", Size: 5000, Mime: "image/jpeg", PutTime: put(now.Add(-time.Hour))},
		{Key: "docs/c.txt", Size: 10, Mime: "text/plain", PutTime: put(now.Add(-30 * time.Minute))},
		{Key: "images/", Dir: true},
	}
	f := searchFilters{Suffixes: []string{"png"}, Mimes: []string{"image/*"}, MinSize: 50, IncludeDirs: false}
	var matched []string
	for _, o := range objs {
		if f.match(o) {
			matched = append(matched, o.Key)
		}
	}
	if len(matched) != 1 || matched[0] != "images/a.png" {
		t.Fatalf("matched = %v, want [images/a.png]", matched)
	}

	// time window with a lower bound only
	f2 := searchFilters{Start: now.Add(-90 * time.Minute), IncludeDirs: false}
	count := 0
	for _, o := range objs {
		if f2.match(o) {
			count++
		}
	}
	if count != 2 {
		t.Errorf("time filter count = %d, want 2", count)
	}

	// directories are included when asked
	f3 := searchFilters{IncludeDirs: true}
	if !f3.match(object{Key: "images/", Dir: true}) {
		t.Error("expected directory to match")
	}
}

func TestBuildListText(t *testing.T) {
	objs := []object{
		{Key: "a.txt", Size: 1024, Hash: "h", PutTime: 123, Mime: "text/plain", Type: 0, EndUser: "u"},
	}
	out := buildListText(objs, "\t", false, true)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if !strings.HasPrefix(lines[0], "Key\tFileSize") {
		t.Errorf("header = %q", lines[0])
	}
	if lines[1] != "a.txt\t1024\th\t123\ttext/plain\t0\tu" {
		t.Errorf("row = %q", lines[1])
	}

	readable := buildListText(objs, ",", true, false)
	if readable != "a.txt,1.00 KB,h,123,text/plain,0,u\n" {
		t.Errorf("readable = %q", readable)
	}
}

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"cdn.example.com/": "https://cdn.example.com",
		"http://x.com":     "http://x.com",
		"https://x.com/":   "https://x.com",
	}
	for in, want := range cases {
		if got := normalizeDomain(in); got != want {
			t.Errorf("normalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrefixHelpers(t *testing.T) {
	if got := normalizePrefix("images"); got != "images/" {
		t.Errorf("normalizePrefix = %q", got)
	}
	if got := parentPrefix("a/b/c/"); got != "a/b/" {
		t.Errorf("parentPrefix = %q", got)
	}
	if got := baseName("a/b/c.txt"); got != "c.txt" {
		t.Errorf("baseName = %q", got)
	}
	if got := baseName("a/b/"); got != "b" {
		t.Errorf("baseName(dir) = %q, want b", got)
	}
	if got := (object{Key: "images/", Dir: true}).Name(); got != "images" {
		t.Errorf("dir Name() = %q, want images", got)
	}
	if got := (object{Key: "a/b/photo.png"}).Name(); got != "photo.png" {
		t.Errorf("file Name() = %q, want photo.png", got)
	}
}

func TestURIEncode(t *testing.T) {
	cases := map[string]string{
		"a b":      "a%20b",
		"a/b":      "a/b",
		"a+b":      "a%2Bb",
		"café.png": "caf%C3%A9.png",
	}
	for in, want := range cases {
		if got := uriEncode(in, false); got != want {
			t.Errorf("uriEncode(%q,false) = %q, want %q", in, got, want)
		}
	}
	if got := uriEncode("a/b", true); got != "a%2Fb" {
		t.Errorf("uriEncode(a/b,true) = %q", got)
	}
}

func TestRegionFromS3Host(t *testing.T) {
	cases := map[string]string{
		"s3.cn-south-1.qiniucs.com": "cn-south-1",
		"s3-cn-east-1.qiniucs.com":  "cn-east-1",
	}
	for in, want := range cases {
		if got := regionFromS3Host(in); got != want {
			t.Errorf("regionFromS3Host(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPresignGET(t *testing.T) {
	ep := s3Endpoint{Host: "s3.cn-south-1.qiniucs.com", Region: "cn-south-1"}
	u := presignGET(ep, "AKID", "SECRET", "ixibu", "a/b c.png", 15*time.Minute)
	for _, want := range []string{
		"https://s3.cn-south-1.qiniucs.com/ixibu/a/b%20c.png?",
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		"X-Amz-Credential=AKID%2F",
		"%2Fcn-south-1%2Fs3%2Faws4_request",
		"X-Amz-SignedHeaders=host",
		"X-Amz-Signature=",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("presignGET missing %q in %s", want, u)
		}
	}
}

func TestExtKind(t *testing.T) {
	cases := map[string]string{
		"a/photo.PNG": "image",
		"movie.mp4":   "video",
		"song.flac":   "audio",
		"doc.pdf":     "pdf",
		"data.json":   "json",
		"page.html":   "xml",
		"main.go":     "code",
		"book.xlsx":   "spreadsheet",
		"report.docx": "doc",
		"slides.pptx": "presentation",
		"novel.epub":  "ebook",
		"icons.svgz":  "image",
		"data.jsonld": "json",
		"feed.rss":    "xml",
		"meet.ics":    "calendar",
		"card.vcf":    "contact",
		"mod.wasm":    "app",
		"lib.class":   "app",
		"bundle.zip":  "archive",
		"font.woff2":  "font",
		"setup.dmg":   "archive",
		"app.db":      "database",
		"server.pem":  "key",
		"redis.conf":  "config",
		"run.log":     "log",
		"deploy.sh":   "script",
		"README.md":   "markdown",
		"notes.txt":   "text",
		"unknown.zzz": "other",
		"noext":       "other",
	}
	for in, want := range cases {
		if got := extKind(in); got != want {
			t.Errorf("extKind(%q) = %q, want %q", in, got, want)
		}
	}
	if !isTextKind("json") || isTextKind("doc") || !isInlineKind("image") || isInlineKind("archive") {
		t.Error("isTextKind/isInlineKind misclassified")
	}
}

func TestInPrefix(t *testing.T) {
	cases := []struct {
		key, prefix string
		want        bool
	}{
		{"a/", "a/", false},    // the directory's own marker
		{"a/x", "a/", true},    // a file in it
		{"a/sub/", "a/", true}, // a sub-directory
		{"b/y", "a/", false},   // a sibling
		{"a", "a/", false},     // "a" is not under "a/"
		{"x", "", true},        // anything is under root
		{"a/", "", true},       // a top-level directory
		{"", "a/", false},      // empty key
		{"", "", false},        // empty key at root
	}
	for _, c := range cases {
		if got := inPrefix(c.key, c.prefix); got != c.want {
			t.Errorf("inPrefix(%q,%q) = %v, want %v", c.key, c.prefix, got, c.want)
		}
	}
}

func TestChildDirs(t *testing.T) {
	got := childDirs("a/", []string{"a/", "a/x/", "a/y/", "b/", "a/x/", "", "a"})
	want := []string{"a/x/", "a/y/"}
	if !slices.Equal(got, want) {
		t.Errorf("childDirs(a/) = %v, want %v", got, want)
	}
	got = childDirs("", []string{"a/", "b/", "a/", "", "c"})
	want = []string{"a/", "b/", "c"}
	if !slices.Equal(got, want) {
		t.Errorf("childDirs(root) = %v, want %v", got, want)
	}
	if got := childDirs("a/", nil); len(got) != 0 {
		t.Errorf("childDirs(empty) = %v, want none", got)
	}
}

func TestBuildObjects(t *testing.T) {
	items := []storage.ListItem{
		{Key: "a/", Fsize: 0}, // own marker, dropped
		{Key: "a/x.png", Fsize: 10},
		{Key: "a/sub/", Fsize: 0},     // directory marker item
		{Key: "b/sib.png", Fsize: 20}, // sibling, dropped
		{Key: "", Fsize: 0},           // empty, dropped
	}
	prefixes := []string{"a/child/", "b/other/", "a/", "a/child/"}
	objs := buildObjects("a/", items, prefixes)
	keys := make([]string, 0, len(objs))
	for _, o := range objs {
		keys = append(keys, o.Key)
		if o.Key == "a/" {
			t.Errorf("own marker leaked into objects")
		}
		if strings.HasPrefix(o.Key, "b/") {
			t.Errorf("sibling %q leaked into objects", o.Key)
		}
	}
	sort.Strings(keys)
	want := []string{"a/child/", "a/sub/", "a/x.png"}
	if !slices.Equal(keys, want) {
		t.Errorf("buildObjects keys = %v, want %v", keys, want)
	}
	for _, o := range objs {
		if (o.Key == "a/child/" || o.Key == "a/sub/") && !o.Dir {
			t.Errorf("%q should be a directory", o.Key)
		}
		if o.Key == "a/x.png" && o.Dir {
			t.Errorf("%q should not be a directory", o.Key)
		}
	}
}

// TestFileRowsFollowData guards the file list's reactivity: the rows must
// follow the directory's objects when they change (the bug where a switch
// showed the previous directory).
func TestFileRowsFollowData(t *testing.T) {
	var data *core.Signal[[]object]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		data = core.NewSignalFunc(cx, []object{{Key: "a/", Dir: true}, {Key: "b/", Dir: true}}, nil)
		return core.View{Style: core.Style{Width: core.Pt(600), Height: core.Pt(300)}, Children: []core.Node{
			core.Scroll{Style: core.Style{FlexGrow: 1}, Children: []core.Node{core.Each(
				func() []object { return data.Get() },
				func(o object) string { return o.Key },
				func(row *core.Scope, item func() object, _ func() int) core.Node {
					return core.Text{Content: item().Name(), TestID: "row"}
				},
			)}},
		}}
	})
	app.Post(func() {
		data.Set([]object{{Key: "eden/2022/", Dir: true}, {Key: "eden/2023/", Dir: true}, {Key: "eden/zip/", Dir: true}})
	})
	app.WaitFor("text:2022", time.Second)
	if !app.Exists("text:2022") || !app.Exists("text:zip") {
		t.Fatal("rows did not follow the new data")
	}
	if app.Exists("text:a") {
		t.Fatal("previous directory's row still shown")
	}
}

// TestRowsFollowItemChange guards that a row built with a reactive
// ContentFunc (reading the item) follows an in-place item change, as the
// upload list's status does.
func TestRowsFollowItemChange(t *testing.T) {
	type task struct {
		ID    int
		Name  string
		State string
	}
	var items *core.Signal[[]task]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		items = core.NewSignalFunc(cx, []task{{1, "a.zip", "等待"}}, nil)
		return core.View{Style: core.Style{Width: core.Pt(500), Height: core.Pt(200)}, Children: []core.Node{core.Each(
			func() []task { return items.Get() },
			func(t task) int { return t.ID },
			func(row *core.Scope, item func() task, _ func() int) core.Node {
				return core.Text{ContentFunc: func() string { return item().State }, TestID: "st"}
			},
		)}}
	})
	app.WaitFor("text:等待", time.Second)
	app.Post(func() { items.Set([]task{{1, "a.zip", "上传中"}}) })
	app.WaitFor("text:上传中", time.Second)
	if app.Exists("text:等待") {
		t.Fatal("status did not update on item change")
	}
}

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"zh-Hans": "zh", "zh_CN": "zh", "en-US": "en", "ja": "ja",
		"KO": "ko", "de-DE": "de", "fr": "fr", "pt-BR": "pt",
		"ru": "ru", "pl": "pl", "fa": "fa", "ar-EG": "ar", "es": "en", "": "en",
	}
	for in, want := range cases {
		if got := normalizeLang(in); got != want {
			t.Errorf("normalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranslate(t *testing.T) {
	if got := translate("zh", "上传"); got != "上传" {
		t.Errorf("zh passthrough = %q", got)
	}
	if got := translate("en", "上传"); got != "Upload" {
		t.Errorf("en 上传 = %q", got)
	}
	if got := translate("ja", "上传"); got != "アップロード" {
		t.Errorf("ja 上传 = %q", got)
	}
	if got := translate("ar", "上传"); got != "رفع" {
		t.Errorf("ar 上传 = %q", got)
	}
	if got := translate("en", "不存在的消息"); got != "不存在的消息" {
		t.Errorf("missing key should fall back, got %q", got)
	}
	if got := translate("xx", "上传"); got != "上传" {
		t.Errorf("unknown lang should fall back, got %q", got)
	}
}

func TestCatalogComplete(t *testing.T) {
	if len(catalog) != 10 {
		t.Fatalf("catalog has %d languages, want 10", len(catalog))
	}
	for _, e := range translations {
		if e.zh == "" {
			t.Fatal("a translation has an empty source")
		}
		for tag, v := range map[string]string{"en": e.en, "ja": e.ja, "ko": e.ko, "de": e.de, "fr": e.fr, "pt": e.pt, "ru": e.ru, "pl": e.pl, "fa": e.fa, "ar": e.ar} {
			if v == "" {
				t.Errorf("missing %s for %q", tag, e.zh)
			}
		}
	}
}
