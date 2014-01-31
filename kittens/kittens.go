package kittens

import (
	"html/template"
	"io"
	"net/http"
	"net/url"
	"time"

	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"appengine/image"
)

type UserUpload struct {
	Name       string
	BlobKey    appengine.BlobKey
	UploadTime time.Time
}

type UserUploadUrl struct {
	Meta UserUpload
	Url  *url.URL
}

func serveError(c appengine.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, "Internal Server Error")
	c.Errorf("%v", err)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	uploadURL, err := blobstore.UploadURL(c, "/upload", nil)
	if err != nil {
		serveError(c, w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	t := template.Must(template.ParseFiles("templates/base.html", "templates/index.html"))
	t.ExecuteTemplate(w, "base", uploadURL)
}

func handleGallery(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	q := datastore.NewQuery("UserUpload")
	var userUploads []UserUpload
	_, err := q.GetAll(c, &userUploads)
	if err != nil {
		c.Errorf("fetching UserUploads: %v", err)
	}
	var uploads [][]UserUploadUrl
	var group []UserUploadUrl
	for i, upload := range userUploads {
		groupCounter := i + 1
		url, err := image.ServingURL(c, upload.BlobKey, nil)
		if err != nil {
			c.Errorf("obtaining url for key %v", upload.BlobKey)
		}
		up := UserUploadUrl{
			Url:  url,
			Meta: upload,
		}
		c.Infof("groupCounter: %v", groupCounter)
		c.Infof("upload: %v", up)
		group = append(group, up)
		if (groupCounter%3) == 0 && groupCounter != 0 {
			c.Infof("group: %v", group)
			uploads = append(uploads, group)
			group = make([]UserUploadUrl, 0)
		} else if groupCounter == len(userUploads) {
			uploads = append(uploads, group)
		}
	}
	c.Infof("%v", uploads)
	context := map[string][][]UserUploadUrl{
		"uploads": uploads,
	}
	w.Header().Set("Content-Type", "text/html")
	t := template.Must(template.ParseFiles("templates/base.html", "templates/gallery.html"))
	t.ExecuteTemplate(w, "base", context)
}

func handleServe(w http.ResponseWriter, r *http.Request) {
	blobstore.Send(w, appengine.BlobKey(r.FormValue("blobKey")))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, others, err := blobstore.ParseUpload(r)
	if err != nil {
		serveError(c, w, err)
		return
	}
	file := blobs["file"]
	if len(file) == 0 {
		c.Errorf("No file uploaded")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	name := others["kitten_name"]
	if len(name) == 0 || name[0] == "" {
		c.Errorf("No kitten name specified")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	blobKey := file[0].BlobKey
	upload := &UserUpload{
		Name:       name[0],
		BlobKey:    blobKey,
		UploadTime: time.Now(),
	}
	key := datastore.NewIncompleteKey(c, "UserUpload", nil)
	c.Infof("key: %s", key)
	_, err = datastore.Put(c, key, upload)
	if err != nil {
		serveError(c, w, err)
		return
	}
	http.Redirect(w, r, "/gallery", http.StatusFound)
}

func init() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/gallery", handleGallery)
	// http.HandleFunc("/serve/", handleServe)
	http.HandleFunc("/upload", handleUpload)
}
