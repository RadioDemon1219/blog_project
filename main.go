package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Post struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

func main() {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/blog"
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("не подключился: %v", err)
	}
	defer pool.Close()

	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		index(w, r, pool, tmpl)
	})

	http.HandleFunc("/posts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listPosts(w, r, pool)
		case http.MethodPost:
			createPost(w, r, pool)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/posts/{id}", func(w http.ResponseWriter, r *http.Request) {

		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		var post Post
		err = pool.QueryRow(r.Context(), "select id,title,body,created_At,status from posts where id=$1", id).Scan(&post.ID, &post.Title, &post.Body, &post.CreatedAt, &post.Status)
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(post)

	})
	newTmpl := template.Must(template.ParseFiles("templates/new.html"))
	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		newForm(w, r, pool, newTmpl)
	})

	log.Println("слушаю на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
func newForm(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, tmpl *template.Template) {
	switch r.Method {
	case http.MethodGet:
		getNewForm(w, r, tmpl)
	case http.MethodPost:
		postNewForm(w, r, pool)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func getNewForm(w http.ResponseWriter, r *http.Request, tmpl *template.Template) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.Execute(w, nil)
	if err != nil {
		serverError(w, err)
		return
	}
}

func postNewForm(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	status := "published"
	if r.FormValue("draft") != "" {
		status = "draft"
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	body := r.FormValue("body")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	_, err := pool.Exec(r.Context(), "insert into posts (title,body,status) values ($1, $2,$3)", title, body, status)
	if err != nil {
		serverError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func index(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, tmpl *template.Template) {
	posts, err := getPosts(r.Context(), pool)
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, posts); err != nil {
		log.Println("template:", err)
	}
}

func listPosts(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	posts, err := getPosts(r.Context(), pool)
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func createPost(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if in.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	var p Post
	err := pool.QueryRow(r.Context(),
		"INSERT INTO posts (title, body, status) VALUES ($1, $2, $3) RETURNING id, title, body, created_at, status",
		in.Title, in.Body, "published",
	).Scan(&p.ID, &p.Title, &p.Body, &p.CreatedAt, &p.Status)
	if err != nil {
		serverError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}
func getPosts(ctx context.Context, pool *pgxpool.Pool) ([]Post, error) {
	rows, err := pool.Query(ctx,
		"SELECT id, title, body, created_at, status FROM posts ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Body, &p.CreatedAt, &p.Status); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func serverError(w http.ResponseWriter, err error) {
	log.Println("error:", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
