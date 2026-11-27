package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// s3Endpoint is a bucket's S3-compatible endpoint, discovered from Qiniu's
// uc service or set by hand.
type s3Endpoint struct {
	Host   string // e.g. s3.cn-south-1.qiniucs.com
	Region string // e.g. cn-south-1
}

// ucHosts is what Qiniu's uc service reports for a bucket.
type ucHosts struct {
	Region  string
	S3Host  string
	S3Alias string
	RsHost  string
	RsfHost string
	IoHost  string
	UpHosts []string
}

// ucQueryHosts asks Qiniu's uc service for a bucket's API hosts.
func ucQueryHosts(ctx context.Context, accessKey, secretKey, bucket string) (ucHosts, error) {
	const path = "/v4/query"
	query := "ak=" + url.QueryEscape(accessKey) + "&bucket=" + url.QueryEscape(bucket)
	u := "https://uc.qbox.me" + path + "?" + query
	sign := hmacSHA1Base64URL(secretKey, path+"?"+query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ucHosts{}, err
	}
	req.Header.Set("Authorization", "QBox "+sign)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ucHosts{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return ucHosts{}, fmt.Errorf("uc query HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		Hosts []struct {
			Region string `json:"region"`
			S3     struct {
				Domains     []string `json:"domains"`
				RegionAlias string   `json:"region_alias"`
			} `json:"s3"`
			Rs struct {
				Domains []string `json:"domains"`
			} `json:"rs"`
			Rsf struct {
				Domains []string `json:"domains"`
			} `json:"rsf"`
			Io struct {
				Domains []string `json:"domains"`
			} `json:"io"`
			Up struct {
				Domains []string `json:"domains"`
			} `json:"up"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return ucHosts{}, err
	}
	if len(out.Hosts) == 0 {
		return ucHosts{}, fmt.Errorf("uc 返回为空")
	}
	h := out.Hosts[0]
	first := func(s []string) string {
		if len(s) == 0 {
			return ""
		}
		return strings.TrimPrefix(strings.TrimPrefix(s[0], "https://"), "http://")
	}
	return ucHosts{
		Region: h.Region, S3Host: first(h.S3.Domains), S3Alias: h.S3.RegionAlias,
		RsHost: first(h.Rs.Domains), RsfHost: first(h.Rsf.Domains), IoHost: first(h.Io.Domains),
		UpHosts: h.Up.Domains,
	}, nil
}

// discoverS3 asks Qiniu's uc service for the S3 endpoint of a bucket.
func discoverS3(ctx context.Context, accessKey, secretKey, bucket string) (s3Endpoint, error) {
	h, err := ucQueryHosts(ctx, accessKey, secretKey, bucket)
	if err != nil {
		return s3Endpoint{}, err
	}
	if h.S3Host == "" {
		return s3Endpoint{}, fmt.Errorf("该空间没有 S3 端点信息")
	}
	region := h.S3Alias
	if region == "" {
		region = regionFromS3Host(h.S3Host)
	}
	return s3Endpoint{Host: h.S3Host, Region: region}, nil
}

// managementSign is the QBox token for a management API request: the
// signature covers "path?query\nbody".
func managementSign(secretKey, path, query, body string) string {
	return hmacSHA1Base64URL(secretKey, path+"?"+query+"\n"+body)
}

// rawList calls the rsf list API directly and returns the request URL and
// the raw response body, for the debug panel.
func rawList(ctx context.Context, rsHost, accessKey, secretKey, bucket, prefix, delimiter, marker string, limit int) (string, string, error) {
	if rsHost == "" {
		return "", "", fmt.Errorf("没有 rsf host（uc 未返回）")
	}
	q := url.Values{}
	q.Set("bucket", bucket)
	if prefix != "" {
		q.Set("prefix", prefix)
	}
	if delimiter != "" {
		q.Set("delimiter", delimiter)
	}
	if marker != "" {
		q.Set("marker", marker)
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	query := q.Encode()
	path := "/list"
	reqURL := "https://" + rsHost + path + "?" + query

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return reqURL, "", err
	}
	req.Header.Set("Authorization", "QBox "+accessKey+":"+managementSign(secretKey, path, query, ""))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return reqURL, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return reqURL, string(body), fmt.Errorf("HTTP %s", resp.Status)
	}
	return reqURL, string(body), nil
}

func regionFromS3Host(host string) string {
	s := host
	s = strings.TrimSuffix(s, ".qiniucs.com")
	s = strings.TrimPrefix(s, "s3.")
	s = strings.TrimPrefix(s, "s3-")
	return s
}

// presignGET returns an AWS Signature V4 presigned URL to download key
// from bucket over the S3-compatible endpoint. ttl is how long it stays
// valid.
func presignGET(ep s3Endpoint, accessKey, secretKey, bucket, key string, ttl time.Duration) string {
	if ttl <= 0 || ttl > 7*24*time.Hour {
		ttl = time.Hour
	}
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	scope := dateStamp + "/" + ep.Region + "/s3/aws4_request"

	canonicalURI := "/" + uriEncode(bucket, false) + "/" + uriEncode(key, false)

	params := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    accessKey + "/" + scope,
		"X-Amz-Date":          amzDate,
		"X-Amz-Expires":       fmt.Sprintf("%d", int(ttl.Seconds())),
		"X-Amz-SignedHeaders": "host",
	}
	names := make([]string, 0, len(params))
	for k := range params {
		names = append(names, k)
	}
	sort.Strings(names)
	var qb strings.Builder
	for i, k := range names {
		if i > 0 {
			qb.WriteByte('&')
		}
		qb.WriteString(uriEncode(k, true))
		qb.WriteByte('=')
		qb.WriteString(uriEncode(params[k], true))
	}
	canonicalQuery := qb.String()

	canonicalHeaders := "host:" + ep.Host + "\n"
	signedHeaders := "host"
	canonicalRequest := strings.Join([]string{
		"GET", canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, "UNSIGNED-PAYLOAD",
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, scope, hexSHA256(canonicalRequest),
	}, "\n")

	signingKey := hmacSHA256(
		hmacSHA256(hmacSHA256(hmacSHA256([]byte("AWS4"+secretKey), dateStamp), ep.Region), "s3"),
		"aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	return "https://" + ep.Host + canonicalURI + "?" + canonicalQuery + "&X-Amz-Signature=" + signature
}

// uriEncode is AWS's RFC 3986 encoding; with encodeSlash false the "/" of
// a path stays.
func uriEncode(s string, encodeSlash bool) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case strings.IndexByte(unreserved, c) >= 0:
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func hexSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hmacSHA1Base64URL(secret, data string) string {
	h := hmac.New(sha1.New, []byte(secret))
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
