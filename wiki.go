// Runs an HTTP server that serves a simple wiki-style app.
package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

var templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")
var interPageLink = regexp.MustCompile(`\[([a-zA-Z0-9]*)\]`)

// Page represents a single page/article in this wiki.
type Page struct {
	Title    string        // page title
	Body     []byte        // page body
	DispBody template.HTML // page body in displayable form (i.e., links expanded out)
	FromSave bool          // whether or not this page object was created following a save operation
}

// Page.save() saves a Page's title and body into a simple text file in the data/ folder.
func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

// loadPage takes in a target title and looks for the desired page in the data folder. If successful, a Page object
// is returned with the target data. If not successful, the Page return value is nil, and an error is instead returned.
func loadPage(title string) (*Page, error) {
	filename := "data/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	dispBody := template.HTML(interPageLink.ReplaceAllFunc(body, func(match []byte) []byte {
		name := string(match[1 : len(match)-1]) // remove opening and closing brackets
		return []byte(fmt.Sprintf("<a href=\"/view/%s\">%s</a>", name, name))
	}))
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body, DispBody: dispBody}, nil
}

// renderTemplate renders the desired template using Page data and writes the resulting response into a ResponseWriter.
func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// viewHandler handles view requests. If the Page exists, the view template is rendered. If it does not exist,
// the handler redirects to the edit endpoint.
func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)

	// show success message for save
	q := r.URL.Query()
	b := q.Get("from_save")
	if b == "true" {
		p.FromSave = true
	}

	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

// editHandler handles page edit requests. A Page object is used to render the edit template. If the Page does not
// exist, the Page.Body component will be empty.
func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

// saveHandler handles requests to save an edited page to the file. Only POST requests are accepted. The user
// is then redirected to the viewHandler upon a successful save.
func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	// handle direct access to URL.
	if r.Method != http.MethodPost {
		http.Error(w, "400 - Bad method type", http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	pg := &Page{Title: title, Body: []byte(body)}
	err := pg.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title+"?from_save=true", http.StatusFound)
}

// makeHandler converts existing functions that accept (w http.ResponseWriter, r *http.Request, title string) into
// handlers compatible with http.HandlerFunc, with parameters (w http.ResponseWriter, r *http.Request), by parsing out
// the name of the desired file/page title from the requests's URL and passing it into these existing functions.
func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

// redirFrontPage redirects automatically to the FrontPage via a 302 redirect. It is meant to handle requests to the
// web root ("/") and is compatible with http.HandlerFunc.
func redirFrontPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/view/FrontPage", http.StatusTemporaryRedirect)
}

// main registers the handlers and executes the HTTP server.
func main() {
	http.HandleFunc("/", redirFrontPage)
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
