package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"todo-app/components"
)

func TestTodoStoreAdd(t *testing.T) {
	store := NewTodoStore()
	texts := []string{"first", "second", "third"}

	for i, text := range texts {
		got := store.Add(text)
		want := components.Todo{ID: i + 1, Text: text, Completed: false}
		if got != want {
			t.Fatalf("Add(%q) = %+v, want %+v", text, got, want)
		}
	}

	wantAll := []components.Todo{
		{ID: 1, Text: "first", Completed: false},
		{ID: 2, Text: "second", Completed: false},
		{ID: 3, Text: "third", Completed: false},
	}
	if got := store.GetAll(); !reflect.DeepEqual(got, wantAll) {
		t.Fatalf("GetAll() = %+v, want %+v", got, wantAll)
	}
}

func TestTodoStoreAddTrimsWhitespace(t *testing.T) {
	store := NewTodoStore()

	got := store.Add("  hello  ")
	want := components.Todo{ID: 1, Text: "hello", Completed: false}
	if got != want {
		t.Fatalf("Add trims surrounding whitespace: got %+v, want %+v", got, want)
	}

	if gotAll := store.GetAll(); !reflect.DeepEqual(gotAll, []components.Todo{want}) {
		t.Fatalf("GetAll() = %+v, want %+v", gotAll, []components.Todo{want})
	}
}

func TestTodoStoreAddRejectsBlankText(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "whitespace only", input: "   \t\n  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewTodoStore()
			store.Add(tt.input)

			if got := store.GetAll(); len(got) != 0 {
				t.Fatalf("Add(%q) created todos %+v, want no todos", tt.input, got)
			}

			got := store.Add("next")
			want := components.Todo{ID: 1, Text: "next", Completed: false}
			if got != want {
				t.Fatalf("Add after rejecting blank text = %+v, want %+v", got, want)
			}
		})
	}
}

func TestTodosHandlerRejectsWhitespaceOnlyText(t *testing.T) {
	previousStore := store
	store = NewTodoStore()
	defer func() { store = previousStore }()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		text := strings.TrimSpace(r.FormValue("text"))
		if text == "" {
			http.Error(w, "Todo text is required", http.StatusBadRequest)
			return
		}

		todo := store.Add(text)
		component := components.TodoItem(todo)
		templ.Handler(component).ServeHTTP(w, r)
	})

	form := url.Values{"text": {"   "}}
	req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /todos with whitespace-only text status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	if got := store.GetAll(); len(got) != 0 {
		t.Fatalf("POST /todos with whitespace-only text created todos %+v, want no todos", got)
	}
}

func TestTodoStoreToggle(t *testing.T) {
	tests := []struct {
		name          string
		toggleIDs     []int
		wantReturnNil bool
		wantTodos     []components.Todo
	}{
		{
			name:      "existing ID toggles from false to true",
			toggleIDs: []int{1},
			wantTodos: []components.Todo{
				{ID: 1, Text: "first", Completed: true},
				{ID: 2, Text: "second", Completed: false},
			},
		},
		{
			name:      "toggling same ID twice flips back to false",
			toggleIDs: []int{1, 1},
			wantTodos: []components.Todo{
				{ID: 1, Text: "first", Completed: false},
				{ID: 2, Text: "second", Completed: false},
			},
		},
		{
			name:          "unknown ID returns nil",
			toggleIDs:     []int{99},
			wantReturnNil: true,
			wantTodos: []components.Todo{
				{ID: 1, Text: "first", Completed: false},
				{ID: 2, Text: "second", Completed: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore([]string{"first", "second"})

			var got *components.Todo
			for _, id := range tt.toggleIDs {
				got = store.Toggle(id)
			}

			if tt.wantReturnNil {
				if got != nil {
					t.Fatalf("Toggle() = %+v, want nil", *got)
				}
			} else {
				if got == nil {
					t.Fatal("Toggle() = nil, want todo")
				}
				wantLast := tt.wantTodos[0]
				if *got != wantLast {
					t.Fatalf("Toggle() returned %+v, want %+v", *got, wantLast)
				}
			}

			if gotAll := store.GetAll(); !reflect.DeepEqual(gotAll, tt.wantTodos) {
				t.Fatalf("GetAll() after Toggle = %+v, want %+v", gotAll, tt.wantTodos)
			}
		})
	}
}

func TestTodoStoreDelete(t *testing.T) {
	tests := []struct {
		name      string
		deleteID  int
		wantFound bool
		wantTodos []components.Todo
	}{
		{
			name:      "deleting existing ID returns true and removes todo",
			deleteID:  1,
			wantFound: true,
			wantTodos: []components.Todo{{ID: 2, Text: "second", Completed: false}, {ID: 3, Text: "third", Completed: false}},
		},
		{
			name:      "deleting unknown ID returns false",
			deleteID:  99,
			wantFound: false,
			wantTodos: []components.Todo{{ID: 1, Text: "first", Completed: false}, {ID: 2, Text: "second", Completed: false}, {ID: 3, Text: "third", Completed: false}},
		},
		{
			name:      "deleting middle todo preserves remaining order",
			deleteID:  2,
			wantFound: true,
			wantTodos: []components.Todo{{ID: 1, Text: "first", Completed: false}, {ID: 3, Text: "third", Completed: false}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore([]string{"first", "second", "third"})

			if got := store.Delete(tt.deleteID); got != tt.wantFound {
				t.Fatalf("Delete(%d) = %v, want %v", tt.deleteID, got, tt.wantFound)
			}

			if got := store.GetAll(); !reflect.DeepEqual(got, tt.wantTodos) {
				t.Fatalf("GetAll() after Delete = %+v, want %+v", got, tt.wantTodos)
			}
		})
	}
}

func TestTodoStoreGetAllInsertionOrder(t *testing.T) {
	store := newTestStore([]string{"first", "second", "third"})

	want := []components.Todo{
		{ID: 1, Text: "first", Completed: false},
		{ID: 2, Text: "second", Completed: false},
		{ID: 3, Text: "third", Completed: false},
	}

	got := store.GetAll()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAll() = %+v, want %+v", got, want)
	}

	got[0].Text = "changed"
	got[1].Completed = true

	if gotAgain := store.GetAll(); !reflect.DeepEqual(gotAgain, want) {
		t.Fatalf("GetAll() after mutating returned slice = %+v, want %+v", gotAgain, want)
	}
}

func newTestStore(texts []string) *TodoStore {
	store := NewTodoStore()
	for _, text := range texts {
		store.Add(text)
	}
	return store
}
