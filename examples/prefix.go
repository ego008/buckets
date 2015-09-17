package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/joyrexus/buckets"
	mux "github.com/julienschmidt/httprouter"
)

const verbose = false // if `true` you'll see log output

func main() {
	// Open the database.
	bx, _ := buckets.Open(tempFilePath())
	defer os.Remove(bx.Path())
	defer bx.Close()

	// Create a bucket for storing todos.
	bucket, _ := bx.New([]byte("todos"))

	// Create our service for handling routes.
	service := NewService(bucket)

	// Create and setup our router.
	router := mux.New()
	router.GET("/:day", service.get)
	router.POST("/:day", service.post)

	// Start our web server.
	srv := httptest.NewServer(router)
	defer srv.Close()

	// Daily todos for client to post.
	posts := []*Todo{
		&Todo{Day: "mon", Task: "milk cows"},
		&Todo{Day: "mon", Task: "feed cows"},
		&Todo{Day: "mon", Task: "wash cows"},
		&Todo{Day: "tue", Task: "wash laundry"},
		&Todo{Day: "tue", Task: "fold laundry"},
		&Todo{Day: "tue", Task: "iron laundry"},
		&Todo{Day: "wed", Task: "flip burgers"},
		&Todo{Day: "thu", Task: "join army"},
		&Todo{Day: "fri", Task: "kill time"},
		&Todo{Day: "sat", Task: "have beer"},
		&Todo{Day: "sat", Task: "make merry"},
		&Todo{Day: "sun", Task: "take aspirin"},
		&Todo{Day: "sun", Task: "pray quietly"},
	}

	// Create our client.
	client := new(Client)

	// Have our client post each daily todo.
	for _, todo := range posts {
		url := srv.URL + "/" + todo.Day
		if err := client.post(url, todo); err != nil {
			fmt.Printf("client post error: %v", err)
		}
	}

	// Have our client get a list of tasks for each day.
	week := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	for _, day := range week {
		url := srv.URL + "/" + day
		tasks, err := client.get(url)
		if err != nil {
			fmt.Printf("client get error: %v", err)
		}
		fmt.Printf("%s: %s\n", day, tasks)
	}

	// Output:
	// mon: milk cows, feed cows, wash cows
	// tue: wash laundry, fold laundry, iron laundry
	// wed: flip burgers
	// thu: join army
	// fri: kill time
	// sat: have beer, make merry
	// sun: take aspirin, pray quietly
}

/* -- MODELS --*/

// A Todo models a daily task.
type Todo struct {
	Task    string    // task to be done
	Day     string    // day to do task
	Created time.Time // when created
}

/* -- SERVICE -- */

// NewService initializes a new instance of our service.
func NewService(bk *buckets.Bucket) *Service {
	prefix := map[string]*buckets.PrefixScanner{
		"/mon": bk.NewPrefixScanner([]byte("/mon")),
		"/tue": bk.NewPrefixScanner([]byte("/tue")),
		"/wed": bk.NewPrefixScanner([]byte("/wed")),
		"/thu": bk.NewPrefixScanner([]byte("/thu")),
		"/fri": bk.NewPrefixScanner([]byte("/fri")),
		"/sat": bk.NewPrefixScanner([]byte("/sat")),
		"/sun": bk.NewPrefixScanner([]byte("/sun")),
	}
	return &Service{bk, prefix}
}

// This service handles requests for todo items.  The items are stored
// in a todos bucket.  The request URLs are used as bucket keys and the
// raw json payload as values.
//
// In MVC parlance, our service would be called a "controller".  We use
// it to define "handle" methods for our router. Note that since we're using
// `httprouter` (abbreviated as `mux` when imported) as our router, each
// service method is a `httprouter.Handle` rather than a `http.HandlerFunc`.
type Service struct {
	todos  *buckets.Bucket
	prefix map[string]*buckets.PrefixScanner
}

// A TaskList is a list of tasks for a particular day.
type TaskList struct {
	Day   string
	Tasks []string
}

// get handles get requests for a particular day, returning the day's
// task list.
func (s *Service) get(w http.ResponseWriter, r *http.Request, _ mux.Params) {
	day := r.URL.String()
	items, err := s.prefix[day].Items()
	if err != nil {
		http.Error(w, err.Error(), 500)
	}

	taskList := &TaskList{day, []string{}}

	for _, item := range items {
		todo, err := decode(item.Value)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		taskList.Tasks = append(taskList.Tasks, todo.Task)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(taskList)
}

// post handles post requests to create a daily todo item.
func (s *Service) post(w http.ResponseWriter, r *http.Request, _ mux.Params) {
	// Read request body's json payload into buffer.
	b, err := ioutil.ReadAll(r.Body)
	todo, err := decode(b)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}

	// Use the day (url path) + creation time as key.
	key := fmt.Sprintf("%s/%s", r.URL, todo.Created.Format(time.RFC3339Nano))

	// Put key/buffer into todos bucket.
	if err := s.todos.Put([]byte(key), b); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if verbose {
		log.Printf("server: %s: %v", key, todo.Task)
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "put todo for %s: %s\n", key, todo)
}

/* -- CLIENT -- */

// Our http client for sending requests.
type Client struct{}

// post sends a post request with a json payload.
func (c *Client) post(url string, todo *Todo) error {
	todo.Created = time.Now()
	bodyType := "application/json"
	body, err := encode(todo)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, bodyType, body)
	if err != nil {
		log.Print(err)
	}
	if verbose {
		log.Printf("client: %s\n", resp.Status)
	}
	return nil
}

// get sends get requests and expects responses to be a json-encoded
// task list.
func (c *Client) get(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	taskList := new(TaskList)
	if err = json.NewDecoder(resp.Body).Decode(taskList); err != nil {
		return "", err
	}
	return strings.Join(taskList.Tasks, ", "), nil
}

/* -- CODEC -- */

// encode marshals a Todo into a buffer.
func encode(todo *Todo) (*bytes.Buffer, error) {
	b, err := json.Marshal(todo)
	if err != nil {
		return &bytes.Buffer{}, err
	}
	return bytes.NewBuffer(b), nil
}

// decode unmarshals a json-encoded byteslice into a Todo.
func decode(b []byte) (*Todo, error) {
	todo := new(Todo)
	if err := json.Unmarshal(b, todo); err != nil {
		return &Todo{}, err
	}
	return todo, nil
}

/* -- UTILITY FUNCTIONS -- */

// tempFilePath returns a temporary file path.
func tempFilePath() string {
	f, _ := ioutil.TempFile("", "bolt-")
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	if err := os.Remove(f.Name()); err != nil {
		log.Fatal(err)
	}
	return f.Name()
}