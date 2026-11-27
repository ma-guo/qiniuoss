package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
)

// client wraps the Qiniu SDK for the operations the file manager uses.
type client struct {
	mac *auth.Credentials
	cfg *storage.Config
	bm  *storage.BucketManager
	up  *storage.FormUploader
}

func newClient(a account) *client {
	mac := auth.New(a.AK, a.SK)
	cfg := storage.NewConfig()
	cfg.UseHTTPS = true
	return &client{
		mac: mac,
		cfg: cfg,
		bm:  storage.NewBucketManager(mac, cfg),
		up:  storage.NewFormUploader(cfg),
	}
}

func (c *client) buckets() ([]string, error) {
	return c.bm.Buckets(false)
}

func (c *client) domains(bucket string) ([]string, error) {
	info, err := c.bm.ListBucketDomains(bucket)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(info))
	for _, d := range info {
		if u := normalizeDomain(d.Domain); u != "" {
			out = append(out, u)
		}
	}
	return out, nil
}

func (c *client) list(ctx context.Context, bucket, prefix, delimiter, marker string, limit int) ([]object, []string, string, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	ret, hasNext, err := c.bm.ListFilesWithContext(ctx, bucket,
		storage.ListInputOptionsPrefix(prefix),
		storage.ListInputOptionsDelimiter(delimiter),
		storage.ListInputOptionsMarker(marker),
		storage.ListInputOptionsLimit(limit))
	if err != nil {
		return nil, nil, "", false, err
	}
	items, prefixes, next := ret.Items, ret.CommonPrefixes, ret.Marker
	objects := buildObjects(prefix, items, prefixes)
	return objects, prefixes, next, hasNext, nil
}

// inPrefix reports whether key is a real child of the directory prefix:
// under it and not the prefix itself (its own marker object).
func inPrefix(key, prefix string) bool {
	if key == "" || key == prefix {
		return false
	}
	return prefix == "" || strings.HasPrefix(key, prefix)
}

// buildObjects turns a listing into the row objects for a directory,
// dropping the prefix's own marker and anything outside the prefix, so a
// listing never shows the directory itself or a sibling.
func buildObjects(prefix string, items []storage.ListItem, prefixes []string) []object {
	objects := make([]object, 0, len(items)+len(prefixes))
	seen := make(map[string]bool, len(items)+len(prefixes))
	for _, it := range items {
		if !inPrefix(it.Key, prefix) {
			continue
		}
		seen[it.Key] = true
		objects = append(objects, object{
			Key:     it.Key,
			Size:    it.Fsize,
			Hash:    it.Hash,
			Mime:    it.MimeType,
			PutTime: it.PutTime,
			Type:    it.Type,
			EndUser: it.EndUser,
			Dir:     strings.HasSuffix(it.Key, "/"),
		})
	}
	for _, p := range prefixes {
		if !inPrefix(p, prefix) || seen[p] {
			continue
		}
		seen[p] = true
		objects = append(objects, object{Key: p, Dir: true})
	}
	return objects
}

func (c *client) stat(bucket, key string) (object, error) {
	fi, err := c.bm.Stat(bucket, key)
	if err != nil {
		return object{}, err
	}
	return object{Key: key, Size: fi.Fsize, Hash: fi.Hash, Mime: fi.MimeType, PutTime: fi.PutTime, Type: fi.Type, Dir: strings.HasSuffix(key, "/")}, nil
}

func (c *client) remove(bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	ops := make([]string, 0, len(keys))
	for _, key := range keys {
		ops = append(ops, storage.URIDelete(bucket, key))
	}
	return c.runBatch(ops)
}

func (c *client) move(srcBucket, srcKey, dstBucket, dstKey string) error {
	return c.runBatch([]string{storage.URIMove(srcBucket, srcKey, dstBucket, dstKey, true)})
}

func (c *client) copy(srcBucket, srcKey, dstBucket, dstKey string) error {
	return c.runBatch([]string{storage.URICopy(srcBucket, srcKey, dstBucket, dstKey, true)})
}

func (c *client) copyMoveMany(copyOp bool, srcBucket, dstBucket string, keys []string, newKey func(string) string) error {
	ops := make([]string, 0, len(keys))
	for _, key := range keys {
		dst := newKey(key)
		if copyOp {
			ops = append(ops, storage.URICopy(srcBucket, key, dstBucket, dst, true))
		} else {
			ops = append(ops, storage.URIMove(srcBucket, key, dstBucket, dst, true))
		}
	}
	return c.runBatch(ops)
}

func (c *client) runBatch(ops []string) error {
	if len(ops) == 0 {
		return nil
	}
	const chunk = 1000
	for len(ops) > 0 {
		n := len(ops)
		if n > chunk {
			n = chunk
		}
		rets, err := c.bm.Batch(ops[:n])
		if err != nil {
			return err
		}
		for i, ret := range rets {
			if ret.Code != 200 {
				msg := fmt.Sprintf("操作 %d 失败，状态码 %d", i+1, ret.Code)
				if ret.Data.Error != "" {
					msg += "：" + ret.Data.Error
				}
				return fmt.Errorf("%s", msg)
			}
		}
		ops = ops[n:]
	}
	return nil
}

// transferPrefix copies or moves every object under oldPrefix in bucket to
// newPrefix in dstBucket.
func (c *client) transferPrefix(ctx context.Context, copyOp bool, bucket, dstBucket, oldPrefix, newPrefix string) (int, error) {
	marker := ""
	moved := 0
	for {
		objects, _, next, hasNext, err := c.list(ctx, bucket, oldPrefix, "", marker, 1000)
		if err != nil {
			return moved, err
		}
		ops := make([]string, 0, len(objects))
		for _, o := range objects {
			dst := newPrefix + strings.TrimPrefix(o.Key, oldPrefix)
			if dst == o.Key && bucket == dstBucket {
				continue
			}
			if copyOp {
				ops = append(ops, storage.URICopy(bucket, o.Key, dstBucket, dst, true))
			} else {
				ops = append(ops, storage.URIMove(bucket, o.Key, dstBucket, dst, true))
			}
		}
		if err := c.runBatch(ops); err != nil {
			return moved, err
		}
		moved += len(ops)
		if moved > 5000 {
			return moved, fmt.Errorf("目录对象超过 5000 个，已停止")
		}
		if !hasNext || next == "" || next == marker {
			break
		}
		marker = next
	}
	return moved, nil
}

func (c *client) movePrefix(ctx context.Context, bucket, oldPrefix, newPrefix string) (int, error) {
	return c.transferPrefix(ctx, false, bucket, bucket, oldPrefix, newPrefix)
}

// upload puts f at bucket/key. progress is the SDK's OnProgress: it
// receives the total file size first, then the bytes uploaded so far.
func (c *client) upload(ctx context.Context, bucket, key string, f core.File, progress func(fsize, uploaded int64)) error {
	policy := storage.PutPolicy{Scope: bucket + ":" + key, Expires: 3600}
	token := policy.UploadToken(c.mac)
	extra := &storage.PutExtra{MimeType: f.MIME, OnProgress: progress, TryTimes: 3}
	var ret storage.PutRet
	if f.Path != "" {
		return c.up.PutFile(ctx, &ret, token, key, f.Path, extra)
	}
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return c.up.Put(ctx, &ret, token, key, r, f.Size, extra)
}

func (c *client) makeFolder(ctx context.Context, bucket, key string) error {
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	policy := storage.PutPolicy{Scope: bucket + ":" + key, Expires: 3600}
	token := policy.UploadToken(c.mac)
	var ret storage.PutRet
	extra := &storage.PutExtra{MimeType: "application/qiniu-object-manager"}
	return c.up.Put(ctx, &ret, token, key, bytes.NewReader(nil), 0, extra)
}

func (c *client) privateURL(domain, key string, ttl time.Duration) string {
	if ttl <= 0 {
		ttl = time.Hour
	}
	deadline := time.Now().Add(ttl).Unix()
	return storage.MakePrivateURLv2(c.mac, normalizeDomain(domain), key, deadline)
}

func (c *client) downloadToFile(ctx context.Context, url, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return c.download(ctx, url, f)
}

func (c *client) download(ctx context.Context, url string, w io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("下载失败：HTTP %s", resp.Status)
	}
	return io.Copy(w, resp.Body)
}

func (c *client) fetchText(ctx context.Context, url string, max int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max))
	if err != nil {
		return "", err
	}
	if int64(len(b)) >= max {
		return string(b) + "\n\n…（内容过长，仅显示前 " + humanSize(max) + "）", nil
	}
	return string(b), nil
}
