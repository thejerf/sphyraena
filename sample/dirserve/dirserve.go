package dirserve

// FIXME: Reconsider the rules on how this works. It is weird that to
// forward the root dir, you need to give this the path of "".

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw"
)

// FIXME: This needs a "debug" mode where we can get more errors out, since
// this is more persnickety than the default server.

// A FileSystemServe serves the target directory in accordance with the
// criteria specified in the package documentation, using net/http's
// FileSystem interface.
//
// Filesystem is a net/http.FileSystem to serve.
//
// IndexFile is what file to serve for the bare directory as a URL, if it
// is present. Normally this is index.html, but this defaults to blank, and
// if blank, no index file will be served.
//
// Index determines whether the directory will serve an automatic index if
// no IndexFile is set, or no IndexFile is found. This defaults to
// false. If neither IndexFile nor Index is present, a direct access of the
// directory will produce a 404.
//
// ServeSubdirectories determines whether this will serve the
// subdirectories of this directory.
//
// ShowFile is a function that will be called on the file name to determine
// whether to show the file. nil defaults to ConservativeFileServing, see
// docs on that below. If the bool is false, the file will be hidden from
// the index (if any) and the file will not be served. If true, the string
// will be used as the MIME type to serve. This server will not guess. If
// left blank, this will be served with Content-Disposition: attachment,
// meaning it will download when accessed instead of display in the
// browser.
//
// Because X-Content-Type-Options will be set to nosniff unless you
// override it with router clauses, it is important
// to get the MIME types correct. See, for instance:
// https://msdn.microsoft.com/en-us/library/ie/gg622941%28v=vs.85%29.aspx
// Bear in mind that IE 8 and early have trouble with sensible JS MIME
// types, so you may want to go ahead and send the deprecated
// "text/javascript" for that, as this module does.
//
// Legal mask is a set of bits for the file's os.FileMode. If any other
// bits but these bits are set to true in the os.FileMode for a file, it
// will not be served. If left to the zero value, it will default to
// allowing user, group, and world read and write, but not execute (0666),
// and no other bits.
type FileSystemServer struct {
	FileSystem          http.FileSystem
	Index               bool
	IndexFile           string
	ServeSubdirectories bool
	// FIXME: ShowFile needs a better name for whether it hides and what
	// the content type is, or maybe this is just two separate functions
	ShowFile  func(string) (bool, string)
	LegalMask os.FileMode

	// This turns off the SendFile optimization used by Go. We encountered
	// this shockingly quickly for something that Go appears to have no
	// controls for...
	BypassSendFile bool
}

func (fss *FileSystemServer) MayStream() bool {
	return false
}

func (fss *FileSystemServer) ServeStreaming(rw *sphyrw.SphyraenaResponseWriter, context *context.Context) {
	if len(context.RemainingPath) > 0 {
		if context.RemainingPath != path.Clean(context.RemainingPath) {
			fmt.Println("Does not match cleaned path")
			http.Error(rw, "Invalid request", 400)
			return
		}
		// FIXME: Need to do some exhaustive testing here for correctness
		// in index and non-index positions
		if context.RemainingPath[0] == '/' {
			context.RemainingPath = context.RemainingPath[1:]
		}
	}

	if context.RemainingPath == "" && !fss.Index {
		rw.Write([]byte("This is a top-level directory request that needs redir"))
		return
	}

	// FIXME: This can be re-inlined at some point
	fss.serveFile(rw, context)
}

var simpleNameBytes = [256]bool{}

func init() {
	simpleNameBytes['_'] = true
	simpleNameBytes['.'] = true
	simpleNameBytes['-'] = true
	for i := 'a'; i <= 'z'; i++ {
		simpleNameBytes[i] = true
	}
	for i := 'A'; i <= 'Z'; i++ {
		simpleNameBytes[i] = true
	}
	for i := '0'; i <= '9'; i++ {
		simpleNameBytes[i] = true
	}
}

// SimpleName returns true if the name does not start with a period,
// and contains only [A-Za-z0-9_.-]. It's very conservative.
func SimpleName(name string) bool {
	if name[0] == '.' {
		return false
	}
	for _, b := range []byte(name) {
		if !simpleNameBytes[b] {
			return false
		}
	}
	return true
}

// ConservativeFileServing implements an extremely conservative serving
// policy, hiding all files starting with period, that consist of something
// other than [A-Za-z0-9_-.]+, and forcing everything to be downloaded
// rather than setting a MIME type.
func ConservativeFileServing(name string) (show bool, mimetype string) {
	if !SimpleName(name) {
		return false, ""
	}

	return true, ""
}

// WhitelistFileExtensions allows you to specify a set of file extensions
// to serve. This will then only display and serve those, and the MIME
// database in the mime package will be used to determine the correct MIME
// type.
func WhitelistedFileExtensions(extensions ...string) func(string) (bool, string) {
	return func(name string) (show bool, mimetype string) {
		if name[0] == '.' {
			return false, ""
		}

		ext := filepath.Ext(name)

		for _, legalExt := range extensions {
			if ext == legalExt {
				return true, mime.TypeByExtension(ext)
			}
		}
		return false, ""
	}
}

// Downloadable extensions serves up the target extensions as downloadable
// files.
func Downloadable(extensions ...string) func(string) (bool, string) {
	return func(name string) (bool, string) {
		if name[0] == '.' {
			return false, ""
		}

		ext := filepath.Ext(name)

		for _, legalExt := range extensions {
			if ext == legalExt {
				return true, ""
			}
		}
		return false, ""
	}
}

// Or allows you to combine together several functions to specify legal
// files, taking the first function that returns a positive result. For
// instance, you can serve "StandardWebFiles and also mp4s" by setting
// the ShowFile as:
//
//    ShowFile: dirserve.Or(dirserve.StandardWebFiles,
//        dirserve.WhitelistedFileExtensions(".mp4"))
func Or(fs ...func(string) (bool, string)) func(string) (bool, string) {
	return func(name string) (bool, string) {
		for _, f := range fs {
			valid, mime := f(name)
			if valid {
				return valid, mime
			}
		}
		return false, ""
	}
}

// StandardWebFiles is a serving policy designed to serve "standard web
// files", permitting htm, html, css, js, jpg, jpeg, gif, and png. The file
// names must conform to SimpleName's restrictions.
func StandardWebFiles(name string) (bool, string) {
	if !SimpleName(name) {
		return false, ""
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".htm", ".html":
		return true, "text/html"
	case ".css":
		return true, "text/css"
	case ".js":
		// http://stackoverflow.com/questions/359895/what-are-the-most-likely-causes-of-javascript-errors-in-ie8/703590#703590
		// despite being "wrong" it's the only thing that works for IE8
		return true, "text/javascript"
	case ".jpg", ".jpeg":
		return true, "image/jpeg"
	case ".gif":
		return true, "image/gif"
	case ".png":
		return true, "image/png"
	default:
		return false, ""
	}
}

func (fss *FileSystemServer) validMode(fm os.FileMode) bool {
	if fss.LegalMask == 0 {
		return fm&^0666 == 0
	} else {
		return fm&^fss.LegalMask == 0
	}
}

// This is largely serveFile from net/http/fs.go in the standard library,
// suitably modified to work for our docs.
//
// TODO(jbowers): The effort to convert all the w and r to req and
// req.Request wasn't worth it... if I ever port upstream changes, just
// switch to a "w" and an "r" early and let the rest cascade through
func (fss *FileSystemServer) serveFile(rw http.ResponseWriter, context *context.Context) {
	// skip redirecting the index page to the dir for now
	path := context.RemainingPath

	// we are guaranteed "/" is a directory marker by the contract of
	// the http.FileSystem interface
	if !fss.ServeSubdirectories && strings.Contains(path, "/") {
		http.NotFound(rw, context.Request)
		return
	}

	// See if we ought to redirect to the index page
	if len(path) == 0 || path[len(path)-1] == '/' {
		if fss.IndexFile != "" {
			path = path + fss.IndexFile
		} else {
			http.NotFound(rw, context.Request)
			return
		}
	}

	f, err := fss.FileSystem.Open(path)
	if err != nil {
		http.NotFound(rw, context.Request)
		return
	}
	defer f.Close()

	d, err := f.Stat()
	if err != nil {
		http.NotFound(rw, context.Request)
		return
	}

	// FIXME: Redirect to canonical dir path
	// FIXME: handle being a directory with no indexing set

	// If this has an illegal mode, refuse to admit it exists
	if !fss.validMode(d.Mode()) {
		http.NotFound(rw, context.Request)
		return
	}

	// FIXME: Pass it through name validation
	sizeFunc := func() (int64, error) { return d.Size(), nil }
	fss.serveContent(rw, context, d.Name(), d.ModTime(), sizeFunc, f)
}

func (fss *FileSystemServer) serveContent(rw http.ResponseWriter, context *context.Context, name string, modtime time.Time, sizeFunc func() (int64, error), content io.ReadSeeker) {
	// FIXME: checkLastModified
	rangeReq, done := fss.checkETag(rw, context, modtime)
	if done {
		return
	}

	code := http.StatusOK

	// this only uses the content type via the ShowFile function

	var (
		show  bool
		ctype string
	)
	if fss.ShowFile != nil {
		show, ctype = fss.ShowFile(name)
	} else {
		show, ctype = ConservativeFileServing(name)
	}
	if !show {
		http.NotFound(rw, context.Request)
		return
	}
	if ctype == "" {
		// FIXME: Need to securely deal with filename, ensure it's a legal
		// header value
		rw.Header().Set("Content-Disposition", "attachment")
	} else {
		rw.Header().Set("Content-Type", ctype)
	}

	size, err := sizeFunc()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	// handle Content-Range header.
	sendSize := size
	var sendContent io.Reader = content
	if size >= 0 {
		ranges, err := parseRange(rangeReq, size)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if sumRangesSize(ranges) > size {
			// The total number of bytes in all the ranges
			// is larger than the size of the file by
			// itself, so this is probably an attack, or a
			// dumb client.  Ignore the range request.
			ranges = nil
		}
		switch {
		case len(ranges) == 1:
			// RFC 2616, Section 14.16:
			// "When an HTTP message includes the content of a single
			// range (for example, a response to a request for a
			// single range, or to a request for a set of ranges
			// that overlap without any holes), this content is
			// transmitted with a Content-Range header, and a
			// Content-Length header showing the number of bytes
			// actually transferred.
			// ...
			// A response to a request for a single range MUST NOT
			// be sent using the multipart/byteranges media type."
			ra := ranges[0]
			if _, err := content.Seek(ra.start, os.SEEK_SET); err != nil {
				http.Error(rw, err.Error(), http.StatusRequestedRangeNotSatisfiable)
				return
			}
			sendSize = ra.length
			code = http.StatusPartialContent
			rw.Header().Set("Content-Range", ra.contentRange(size))
		case len(ranges) > 1:
			sendSize = rangesMIMESize(ranges, ctype, size)
			code = http.StatusPartialContent

			pr, pw := io.Pipe()
			mw := multipart.NewWriter(pw)
			rw.Header().Set("Content-Type", "multipart/byteranges; boundary="+mw.Boundary())
			sendContent = pr
			defer pr.Close() // cause writing goroutine to fail and exit if CopyN doesn't finish.
			go func() {
				for _, ra := range ranges {
					part, err := mw.CreatePart(ra.mimeHeader(ctype, size))
					if err != nil {
						pw.CloseWithError(err)
						return
					}
					if _, err := content.Seek(ra.start, os.SEEK_SET); err != nil {
						pw.CloseWithError(err)
						return
					}
					if _, err := io.CopyN(part, content, ra.length); err != nil {
						pw.CloseWithError(err)
						return
					}
				}
				mw.Close()
				pw.Close()
			}()
		}

		rw.Header().Set("Accept-Ranges", "bytes")
		if rw.Header().Get("Content-Encoding") == "" {
			rw.Header().Set("Content-Length", strconv.FormatInt(sendSize, 10))
		}
	}

	rw.WriteHeader(code)

	if context.Request.Method != "HEAD" {
		if fss.BypassSendFile {
			_, err := io.CopyN(writerOnly{rw}, sendContent, sendSize)
			if err != nil {
				// FIXME: Log somehow, in context
				fmt.Println("error in sending file:", err)
			}
		} else {
			_, err := io.CopyN(rw, sendContent, sendSize)
			if err != nil {
				// FIXME: Log somehow, in context
				fmt.Println("error in sending file:", err)
			}
		}
	}
}

type writerOnly struct {
	io.Writer
}

func (fss *FileSystemServer) checkETag(rw http.ResponseWriter, context *context.Context, modtime time.Time) (rangeReq string, done bool) {
	etag := rw.Header().Get("Etag")
	rangeReq = context.Request.Header.Get("Range")

	// Invalidate the range request if the entity doesn't match the one
	// the client was expecting.
	// "If-Range: version" means "ignore the Range: header unless version matches the
	// current file."
	// We only support ETag versions.
	// The caller must have set the ETag on the response already.
	if ir := context.Request.Header.Get("If-Range"); ir != "" && ir != etag {
		// The If-Range value is typically the ETag value, but it may also be
		// the modtime date. See golang.org/issue/8367.
		timeMatches := false
		if !modtime.IsZero() {
			if t, err := parseTime(ir); err == nil && t.Unix() == modtime.Unix() {
				timeMatches = true
			}
		}
		if !timeMatches {
			rangeReq = ""
		}
	}

	if inm := context.Request.Header.Get("If-None-Match"); inm != "" {
		// Must know ETag.
		if etag == "" {
			return rangeReq, false
		}

		if context.Request.Method != "GET" && context.Request.Method != "HEAD" {
			return rangeReq, false
		}

		if inm == etag || inm == "*" {
			h := rw.Header()
			delete(h, "Content-Type")
			delete(h, "Content-Length")
			rw.WriteHeader(http.StatusNotModified)
			return "", true
		}
	}
	return rangeReq, false
}

// httpRange specifies the byte range to be sent to the client.
type httpRange struct {
	start, length int64
}

func (r httpRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size)
}

func (r httpRange) mimeHeader(contentType string, size int64) textproto.MIMEHeader {
	return textproto.MIMEHeader{
		"Content-Range": {r.contentRange(size)},
		"Content-Type":  {contentType},
	}
}

// parseRange parses a Range header string as per RFC 2616.
func parseRange(s string, size int64) ([]httpRange, error) {
	if s == "" {
		return nil, nil // header not present
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges []httpRange
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = strings.TrimSpace(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, errors.New("invalid range")
		}
		start, end := strings.TrimSpace(ra[:i]), strings.TrimSpace(ra[i+1:])
		var r httpRange
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file.
			i, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, errors.New("invalid range")
			}
			if i > size {
				i = size
			}
			r.start = size - i
			r.length = size - r.start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i > size || i < 0 {
				return nil, errors.New("invalid range")
			}
			r.start = i
			if end == "" {
				// If no end is specified, range extends to end of the file.
				r.length = size - r.start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.start > i {
					return nil, errors.New("invalid range")
				}
				if i >= size {
					i = size - 1
				}
				r.length = i - r.start + 1
			}
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

// countingWriter counts how many bytes have been written to it.
type countingWriter int64

func (w *countingWriter) Write(p []byte) (n int, err error) {
	*w += countingWriter(len(p))
	return len(p), nil
}

// rangesMIMESize returns the number of bytes it takes to encode the
// provided ranges as a multipart response.
func rangesMIMESize(ranges []httpRange, contentType string, contentSize int64) (encSize int64) {
	var w countingWriter
	mw := multipart.NewWriter(&w)
	for _, ra := range ranges {
		mw.CreatePart(ra.mimeHeader(contentType, contentSize))
		encSize += ra.length
	}
	mw.Close()
	encSize += int64(w)
	return
}

func sumRangesSize(ranges []httpRange) (size int64) {
	for _, ra := range ranges {
		size += ra.length
	}
	return
}

var timeFormats = []string{
	"Mon, 02 Jan 2006 15:04:05 GMT",
	time.RFC850,
	time.ANSIC,
}

// ParseTime parses a time header (such as the Date: header),
// trying each of the three formats allowed by HTTP/1.1:
// TimeFormat, time.RFC850, and time.ANSIC.
func parseTime(text string) (t time.Time, err error) {
	for _, layout := range timeFormats {
		t, err = time.Parse(layout, text)
		if err == nil {
			return
		}
	}
	return
}
