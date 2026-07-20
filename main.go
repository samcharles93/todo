package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/a-h/templ"

	"todo-app/components"
)

// TodoStore manages the collection of todos with thread-safe operations
type TodoStore struct {
	mu     sync.RWMutex
	todos  []components.Todo
	nextID int
}

// NewTodoStore creates a new todo store
func NewTodoStore() *TodoStore {
	return &TodoStore{
		todos:  make([]components.Todo, 0),
		nextID: 1,
	}
}

// Add adds a new todo to the store
func (s *TodoStore) Add(text string) components.Todo {
	s.mu.Lock()
	defer s.mu.Unlock()

	todo := components.Todo{
		ID:        s.nextID,
		Text:      text,
		Completed: false,
	}
	s.nextID++
	s.todos = append(s.todos, todo)
	return todo
}

// Toggle toggles the completed status of a todo
func (s *TodoStore) Toggle(id int) *components.Todo {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos[i].Completed = !s.todos[i].Completed
			return &s.todos[i]
		}
	}
	return nil
}

// Delete removes a todo from the store
func (s *TodoStore) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			return true
		}
	}
	return false
}

// GetAll returns all todos
func (s *TodoStore) GetAll() []components.Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]components.Todo, len(s.todos))
	copy(result, s.todos)
	return result
}

var store = NewTodoStore()

func countRemainingTodos(todos []components.Todo) int {
	remaining := 0
	for _, todo := range todos {
		if !todo.Completed {
			remaining++
		}
	}
	return remaining
}

func remainingCountText(remaining int) string {
	if remaining == 1 {
		return "1 item left"
	}
	return fmt.Sprintf("%d items left", remaining)
}

func renderComponents(w http.ResponseWriter, r *http.Request, rendered ...templ.Component) {
	var buf bytes.Buffer
	for _, component := range rendered {
		if err := component.Render(r.Context(), &buf); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	_, _ = w.Write(buf.Bytes())
}

func main() {
	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("public"))))

	// Main page route
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		todos := store.GetAll()
		component := components.Page(todos, remainingCountText(countRemainingTodos(todos)))
		templ.Handler(component).ServeHTTP(w, r)
	})

	// Add new todo
	http.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		text := r.FormValue("text")
		if text == "" {
			http.Error(w, "Todo text is required", http.StatusBadRequest)
			return
		}

		todo := store.Add(text)
		todos := store.GetAll()
		renderComponents(w, r,
			components.TodoItem(todo),
			components.RemainingCount(remainingCountText(countRemainingTodos(todos))),
		)
	})

	// Toggle todo
	http.HandleFunc("/todos/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		idStr := r.FormValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid todo ID", http.StatusBadRequest)
			return
		}

		todo := store.Toggle(id)
		if todo == nil {
			http.NotFound(w, r)
			return
		}

		// For HTMX requests, return just the updated component
		if r.Header.Get("Hx-Request") == "true" {
			todos := store.GetAll()
			renderComponents(w, r,
				components.TodoItem(*todo),
				components.RemainingCount(remainingCountText(countRemainingTodos(todos))),
			)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	})

	// Delete todo
	http.HandleFunc("/todos/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract todo ID from path
		path := r.URL.Path
		idStr := path[len("/todos/"):]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid todo ID", http.StatusBadRequest)
			return
		}

		if !store.Delete(id) {
			http.NotFound(w, r)
			return
		}

		todos := store.GetAll()
		renderComponents(w, r,
			templ.Raw(""),
			components.RemainingCount(remainingCountText(countRemainingTodos(todos))),
		)
	})

	fmt.Println("Todo List Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
