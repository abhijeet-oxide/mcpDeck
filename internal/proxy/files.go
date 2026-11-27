package proxy

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// MaxUpload is the largest file a tool may upload (HTTP-3).
const MaxUpload = 20 << 20

// CodeNoFile is a file that isn't in the application's files folder.
const CodeNoFile = "file_not_found"

// FileInfo is one file in an application's files folder. No path: tools
// and agents only ever see names.
type FileInfo struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// ListFiles lists the regular files directly in api's files folder.
func (p *Proxy) ListFiles(ctx context.Context, apiID string) ([]FileInfo, error) {
	api, err := p.store.API(ctx, apiID)
	if err != nil {
		return nil, &Error{Code: CodeNotFound, Message: "API not found"}
	}
	if api.FilesDir == "" {
		return nil, noFolder(api)
	}
	ents, err := os.ReadDir(api.FilesDir)
	if err != nil {
		return nil, &Error{Code: CodeNoFile, Message: fmt.Sprintf("the %s files folder can't be read (%v); a person can pick another on the application's Settings", api.Name, err)}
	}
	out := []FileInfo{}
	for _, e := range ents {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, FileInfo{Name: e.Name(), Size: fi.Size(), Modified: fi.ModTime().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out, nil
}

func noFolder(api store.API) error {
	return &Error{Code: CodeNoFile, Message: "the " + api.Name + " application has no files folder; ask the user to pick one on its Settings in mcpDeck"}
}

// readUpload reads one file by name from api's files folder: a plain name,
// a regular file (no links), at most MaxUpload bytes.
func readUpload(api store.API, name string) ([]byte, error) {
	if api.FilesDir == "" {
		return nil, noFolder(api)
	}
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil, &Error{Code: CodeNoFile, Message: fmt.Sprintf("%q isn't a file name; pass a name from files.list(%q)", name, api.Name)}
	}
	full := filepath.Join(api.FilesDir, name)
	st, err := os.Lstat(full)
	if err != nil || !st.Mode().IsRegular() {
		return nil, &Error{Code: CodeNoFile, Message: fmt.Sprintf("no file %q in the %s files folder; pass a name from files.list(%q)", name, api.Name, api.Name)}
	}
	if st.Size() > MaxUpload {
		return nil, &Error{Code: CodeNoFile, Message: fmt.Sprintf("%s is %d MB; uploads are limited to %d MB", name, st.Size()>>20, MaxUpload>>20)}
	}
	return os.ReadFile(full)
}

// fileRef is a body value that stands for a file: {"$file": "name.pdf"}.
func fileRef(v any) (string, bool) {
	m, ok := v.(map[string]any)
	if !ok || len(m) != 1 {
		return "", false
	}
	n, ok := m["$file"].(string)
	return n, ok
}

func hasFile(m map[string]any) bool {
	for _, v := range m {
		if _, ok := fileRef(v); ok {
			return true
		}
	}
	return false
}

// multipartBody encodes fields as multipart/form-data, reading file
// references from the files folder. It also returns what the call log
// keeps: field names, values that aren't secret-like and file names and
// sizes, never file contents.
func buildMultipart(api store.API, fields map[string]any) (body []byte, contentType string, logged []byte, err error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	log := map[string]any{}
	for _, k := range keys {
		v := fields[k]
		if name, ok := fileRef(v); ok {
			data, err := readUpload(api, name)
			if err != nil {
				return nil, "", nil, err
			}
			ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
			if ct == "" {
				ct = "application/octet-stream"
			}
			h := textproto.MIMEHeader{}
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, k, name))
			h.Set("Content-Type", ct)
			part, _ := w.CreatePart(h)
			_, _ = part.Write(data)
			log[k] = fmt.Sprintf("[file %s, %d bytes, %s]", name, len(data), ct)
			continue
		}
		vals := []any{v}
		if arr, ok := v.([]any); ok {
			vals = arr
		}
		for _, x := range vals {
			_ = w.WriteField(k, fmtArg(x))
		}
		if stateKey.MatchString(k) {
			log[k] = "[masked]"
			continue
		}
		log[k] = v
	}
	_ = w.Close()
	logged = MaskJSON(mustJSON(log))
	return buf.Bytes(), w.FormDataContentType(), logged, nil
}

func isString(v any) bool { _, ok := v.(string); return ok }
