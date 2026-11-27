package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// account is one set of Qiniu credentials, with the CDN domain chosen for
// each bucket that the app shows previews and download links from.
type account struct {
	Name    string            `json:"name"`
	AK      string            `json:"ak"`
	SK      string            `json:"sk"`
	Domains map[string]string `json:"domains,omitempty"`
	// S3AccessKey and S3SecretKey are the S3-compatible keys (created in
	// the Qiniu console) that let objects be read without a bound domain.
	S3AccessKey string `json:"s3_ak,omitempty"`
	S3SecretKey string `json:"s3_sk,omitempty"`
	// S3Endpoint and S3Region override the discovered S3 host; leave empty
	// to discover them from the bucket's region.
	S3Endpoint string `json:"s3_endpoint,omitempty"`
	S3Region   string `json:"s3_region,omitempty"`
}

// object is one entry of a bucket listing. Dir marks a common prefix or a
// directory marker object, whose key ends in "/".
type object struct {
	Key     string
	Size    int64
	Hash    string
	Mime    string
	PutTime int64
	Type    int
	EndUser string
	Dir     bool
}

func (o object) Name() string {
	return baseName(o.Key)
}

// childDirs filters a listing's common prefixes to the direct child
// directories of prefix, dropping the prefix's own marker, duplicates and
// anything outside the prefix, sorted. It is the one place the tree and
// the file list build sub-directories from.
func childDirs(prefix string, raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, p := range raw {
		if !inPrefix(p, prefix) || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// baseName is the last path segment, ignoring a directory's trailing
// slash: "a/b/c.txt" is "c.txt" and "a/b/" is "b".
func baseName(key string) string {
	key = strings.TrimSuffix(key, "/")
	if key == "" {
		return ""
	}
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

func parentPrefix(prefix string) string {
	p := strings.TrimSuffix(prefix, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i+1]
	}
	return ""
}

func joinPrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + key
}

func normalizePrefix(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// humanSize writes a byte count the way qshell's --readable flag does.
func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	for _, u := range units {
		v /= 1024
		if v < 1024 {
			return fmt.Sprintf("%.2f %s", v, u)
		}
	}
	return fmt.Sprintf("%.2f EB", v/1024)
}

func displaySize(n int64, readable bool) string {
	if readable {
		return humanSize(n)
	}
	return strconv.FormatInt(n, 10)
}

// formatPutTime formats Qiniu's 100-nanosecond timestamp in local time.
func formatPutTime(putTime int64) string {
	if putTime <= 0 {
		return "-"
	}
	t := time.Unix(putTime/10_000_000, 0)
	return t.Format("2006-01-02 15:04:05")
}

// parseTime reads the forms qshell's --start and --end accept, plus a few
// friendlier ones. A date without a time is midnight.
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		"2006-01-02-15-04-05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法识别的时间 %q，请使用 yyyy-mm-dd-hh-MM-ss 或 yyyy-mm-dd", s)
}

// parseSize reads bytes, or a number with K/M/G/T (1024-based).
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	mult := int64(1)
	if i := strings.IndexAny(s, "kmgt"); i >= 0 {
		switch s[i] {
		case 'k':
			mult = 1 << 10
		case 'm':
			mult = 1 << 20
		case 'g':
			mult = 1 << 30
		case 't':
			mult = 1 << 40
		}
		s = strings.TrimSpace(s[:i] + s[i+1:])
	}
	s = strings.TrimSuffix(s, "b")
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("无法识别的大小 %q", s)
	}
	return int64(n * float64(mult)), nil
}

func splitList(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '\n' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func parseFileTypes(s string) ([]int, error) {
	var out []int
	for _, f := range splitList(s) {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("无法识别的存储类型 %q", f)
		}
		out = append(out, n)
	}
	return out, nil
}

func matchMime(pattern, mime string) bool {
	p := strings.ToLower(strings.TrimSpace(pattern))
	m := strings.ToLower(mime)
	if p == "" {
		return true
	}
	switch {
	case p == "*" || p == "*/*":
		return true
	case strings.HasSuffix(p, "/*"):
		return strings.HasPrefix(m, strings.TrimSuffix(p, "*"))
	case strings.HasSuffix(p, "/"):
		return strings.HasPrefix(m, p)
	default:
		return m == p || strings.Contains(m, p)
	}
}

type searchFilters struct {
	Prefix      string
	Suffixes    []string
	Mimes       []string
	Types       []int
	MinSize     int64
	MaxSize     int64
	Start       time.Time
	End         time.Time
	IncludeDirs bool
}

func (f searchFilters) match(o object) bool {
	if o.Dir && !f.IncludeDirs {
		return false
	}
	if f.Prefix != "" && !strings.HasPrefix(o.Key, f.Prefix) {
		return false
	}
	if len(f.Suffixes) > 0 {
		lower := strings.ToLower(o.Key)
		ok := false
		for _, s := range f.Suffixes {
			s = strings.ToLower(strings.TrimSpace(s))
			s = strings.TrimPrefix(s, ".")
			if s == "" {
				continue
			}
			if strings.HasSuffix(lower, "."+s) || strings.HasSuffix(lower, s) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(f.Mimes) > 0 {
		ok := false
		for _, p := range f.Mimes {
			if matchMime(p, o.Mime) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(f.Types) > 0 {
		ok := false
		for _, t := range f.Types {
			if t == o.Type {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if f.MinSize > 0 && o.Size < f.MinSize {
		return false
	}
	if f.MaxSize > 0 && o.Size > f.MaxSize {
		return false
	}
	if !f.Start.IsZero() || !f.End.IsZero() {
		t := time.Unix(o.PutTime/10_000_000, 0)
		if !f.Start.IsZero() && t.Before(f.Start) {
			return false
		}
		if !f.End.IsZero() && !t.Before(f.End) {
			return false
		}
	}
	return true
}

// buildListText renders results as qshell listbucket2's tab-separated
// output does, optionally with the header the command writes.
func buildListText(results []object, sep string, readable, header bool) string {
	if sep == "" {
		sep = "\t"
	}
	var b strings.Builder
	if header {
		b.WriteString(strings.Join([]string{"Key", "FileSize", "Hash", "PutTime", "MimeType", "FileType", "EndUser"}, sep))
		b.WriteString("\n")
	}
	for _, o := range results {
		fields := []string{
			o.Key,
			displaySize(o.Size, readable),
			o.Hash,
			strconv.FormatInt(o.PutTime, 10),
			o.Mime,
			strconv.Itoa(o.Type),
			o.EndUser,
		}
		b.WriteString(strings.Join(fields, sep))
		b.WriteString("\n")
	}
	return b.String()
}

func normalizeDomain(d string) string {
	d = strings.TrimSpace(d)
	d = strings.TrimSuffix(d, "/")
	if d == "" {
		return ""
	}
	if !strings.HasPrefix(d, "http://") && !strings.HasPrefix(d, "https://") {
		d = "https://" + d
	}
	return d
}

// objectKind classifies an object into a file-type category used to pick
// its icon and preview. The extension is tried first, then the MIME type.
func objectKind(o object) string {
	if k := extKind(o.Key); k != "other" {
		return k
	}
	if k := mimeKind(o.Mime); k != "other" {
		return k
	}
	return "other"
}

// isTextKind reports whether a category's content is text to show inline.
func isTextKind(kind string) bool {
	switch kind {
	case "text", "code", "json", "xml", "markdown", "config", "log", "script":
		return true
	}
	return false
}

// isInlineKind reports whether a category can be previewed in the dialog.
func isInlineKind(kind string) bool {
	return kind == "image" || kind == "video" || isTextKind(kind)
}

func extKind(key string) string {
	key = strings.ToLower(key)
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "other"
	}
	switch key[i:] {
	// images
	case ".jpg", ".jpeg", ".jpe", ".png", ".gif", ".webp", ".avif", ".bmp", ".tif", ".tiff",
		".svg", ".svgz", ".ico", ".heic", ".heif", ".apng", ".ai":
		return "image"
	// video
	case ".mp4", ".mp4v", ".m4v", ".webm", ".ogv", ".mov", ".avi", ".mkv", ".wmv", ".flv",
		".mpeg", ".mpg", ".3gp", ".3g2", ".m2ts", ".mts":
		return "video"
	// audio
	case ".mp3", ".wav", ".ogg", ".oga", ".opus", ".m4a", ".aac", ".flac", ".mid", ".midi",
		".weba", ".amr", ".aiff", ".aif", ".wma":
		return "audio"
	case ".pdf":
		return "pdf"
	// json
	case ".json", ".jsonld", ".geojson", ".map":
		return "json"
	// xml / markup
	case ".xml", ".html", ".htm", ".xhtml", ".xhtm", ".rss", ".atom":
		return "xml"
	// code
	case ".css", ".scss", ".sass", ".less", ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx", ".vue",
		".go", ".py", ".pyw", ".rb", ".php", ".java", ".kt", ".kts", ".scala", ".rs", ".c", ".h",
		".cc", ".cpp", ".cxx", ".hpp", ".hxx", ".cs", ".swift", ".m", ".mm", ".lua", ".dart", ".r",
		".pl", ".pm", ".ex", ".exs", ".erl", ".hrl", ".hs", ".clj", ".cljs", ".groovy", ".sql",
		".proto", ".graphql", ".gql", ".asm", ".s", ".vb", ".pas", ".fs":
		return "code"
	// spreadsheets
	case ".csv", ".tsv", ".xls", ".xlsx", ".ods", ".numbers":
		return "spreadsheet"
	// documents
	case ".doc", ".docx", ".odt", ".rtf", ".pages", ".wps", ".dot", ".ps", ".eps", ".one":
		return "doc"
	// presentations
	case ".ppt", ".pptx", ".odp", ".dps":
		return "presentation"
	// ebooks
	case ".epub", ".mobi", ".azw", ".azw3", ".fb2":
		return "ebook"
	// archives / disk images
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".tgz", ".bz2", ".tbz2", ".xz", ".txz", ".zst",
		".lz4", ".iso", ".cab", ".dmg":
		return "archive"
	// fonts
	case ".ttf", ".otf", ".woff", ".woff2", ".eot", ".ttc", ".fon":
		return "font"
	// executables / runtimes
	case ".exe", ".msi", ".apk", ".ipa", ".deb", ".rpm", ".pkg", ".bin", ".jar", ".war",
		".class", ".wasm", ".appimage":
		return "app"
	case ".db", ".sqlite", ".sqlite3", ".mdb", ".accdb":
		return "database"
	case ".pem", ".key", ".crt", ".cer", ".p12", ".pfx", ".pub", ".asc", ".gpg":
		return "key"
	case ".ini", ".conf", ".cfg", ".toml", ".yaml", ".yml", ".env", ".properties", ".plist", ".lock":
		return "config"
	case ".log":
		return "log"
	case ".sh", ".bash", ".zsh", ".fish", ".bat", ".cmd", ".ps1", ".psm1", ".vbs":
		return "script"
	case ".md", ".markdown", ".mdx":
		return "markdown"
	case ".ics", ".ical":
		return "calendar"
	case ".vcf", ".vcard":
		return "contact"
	case ".txt", ".text", ".nfo", ".srt", ".vtt", ".cue":
		return "text"
	}
	return "other"
}

func mimeKind(mime string) string {
	m := strings.ToLower(strings.TrimSpace(mime))
	if m == "" {
		return "other"
	}
	switch {
	case strings.HasPrefix(m, "image/"):
		return "image"
	case strings.HasPrefix(m, "video/"):
		return "video"
	case strings.HasPrefix(m, "audio/"):
		return "audio"
	case strings.Contains(m, "json"):
		return "json"
	case strings.Contains(m, "pdf"):
		return "pdf"
	case strings.Contains(m, "zip"), strings.Contains(m, "compress"), strings.Contains(m, "tar"),
		strings.Contains(m, "rar"), strings.Contains(m, "7z"):
		return "archive"
	case strings.Contains(m, "word"), strings.Contains(m, "opendocument.text"), strings.Contains(m, "rtf"):
		return "doc"
	case strings.Contains(m, "excel"), strings.Contains(m, "spreadsheet"), strings.Contains(m, "csv"):
		return "spreadsheet"
	case strings.Contains(m, "presentation"), strings.Contains(m, "powerpoint"):
		return "presentation"
	case strings.HasPrefix(m, "font/"), strings.Contains(m, "font"):
		return "font"
	case strings.Contains(m, "xml"), strings.Contains(m, "html"):
		return "xml"
	case strings.Contains(m, "javascript"), strings.Contains(m, "ecmascript"):
		return "code"
	case strings.HasPrefix(m, "text/"):
		return "text"
	default:
		return "other"
	}
}

func sortObjects(objs []object) {
	sort.SliceStable(objs, func(i, j int) bool {
		a, b := objs[i], objs[j]
		if a.Dir != b.Dir {
			return a.Dir
		}
		return strings.ToLower(a.Key) < strings.ToLower(b.Key)
	})
}

// importQshellAccount reads the account qshell stores in ~/.qshell.
func importQshellAccount() (account, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return account{}, err
	}
	path := filepath.Join(home, ".qshell", "account.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return account{}, err
	}
	text := strings.TrimSpace(string(data))
	// qshell writes "name:ak:sk" on one line.
	if parts := strings.SplitN(text, ":", 3); len(parts) == 3 {
		return account{Name: strings.TrimSpace(parts[0]), AK: strings.TrimSpace(parts[1]), SK: strings.TrimSpace(parts[2])}, nil
	}
	// Some versions write "Name: x\nAccessKey: y\nSecretKey: z".
	var a account
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "name":
			a.Name = v
		case "accesskey":
			a.AK = v
		case "secretkey":
			a.SK = v
		}
	}
	if a.AK == "" || a.SK == "" {
		return account{}, fmt.Errorf("qshell 账户文件格式无法识别")
	}
	if a.Name == "" {
		a.Name = "qshell"
	}
	return a, nil
}
