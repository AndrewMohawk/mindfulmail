package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"

	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library

	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

func main() {
	// Variable to store the database path
	dbPath := "/home/mindfulmail/database/mindfulmail.db"

	// Lets see if the file exists, and if not lets create the database
	_, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		fmt.Println("Database does not exist, creating database")
		// Create the database
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		// Create the users table
		sqlStmt := `
		CREATE TABLE users (id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, email TEXT, created_at DATETIME);
		`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Printf("%q: %s", err, sqlStmt)
			return
		}
	}

	db, err := sql.Open("sqlite3", dbPath)

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	var version string
	err = db.QueryRow("SELECT SQLITE_VERSION()").Scan(&version)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(version)

	key := "Secret-session-key" // Replace with your SESSION_SECRET or similar
	maxAge := 86400 * 30        // 30 days
	isProd := false             // Set to true when serving over https

	store := sessions.NewCookieStore([]byte(key))
	store.MaxAge(maxAge)
	store.Options.Path = "/"
	store.Options.HttpOnly = true // HttpOnly should always be enabled
	store.Options.Secure = isProd

	gothic.Store = store

	goth.UseProviders(
		google.New("173696006804-p4556o3r57m418qh3rvb7f6cdoev8pk2.apps.googleusercontent.com", "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", "http://mindfulmail.net/auth/google/callback", "email", "profile"),
	)

	p := pat.New()
	p.Get("/auth/{provider}/callback", func(res http.ResponseWriter, req *http.Request) {

		user, err := gothic.CompleteUserAuth(res, req)
		if err != nil {
			fmt.Fprintln(res, err)
			return
		}

		//fmt.Println(user)

		// Check if user is already authenticated
		session, err := store.Get(req, "session-name")
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		// if auth, ok := session.Values["authenticated"].(bool); ok && auth {
		// 	// Redirect to success page
		// 	http.Redirect(res, req, "/success", http.StatusSeeOther)
		// 	return
		// }

		// Set session value to mark user as authenticated
		session.Values["userEmail"] = user.Email
		session.Values["authenticated"] = true
		// Lets see if this user already exists in the database
		rows, err := db.Query("SELECT id FROM users WHERE email=?", user.Email)
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		if rows.Next() {
			//fmt.Println("User already exists in database")
		} else {
			//fmt.Println("User does not exist in database, adding user to database")
			// Lets add the user to the database
			stmt, err := db.Prepare("INSERT INTO users(email, created_at) VALUES(?, datetime('now'))")
			if err != nil {
				log.Fatal(err)
			}
			defer stmt.Close()
			_, err = stmt.Exec(user.Email)
			if err != nil {
				log.Fatal(err)
			}
		}

		err = session.Save(req, res)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		// Lets save the user to the database

		// t, _ := template.ParseFiles("templates/success.html")
		// t.Execute(res, user)
		http.Redirect(res, req, "/success", http.StatusSeeOther)
		return
	})

	p.Get("/auth/{provider}", func(res http.ResponseWriter, req *http.Request) {
		gothic.BeginAuthHandler(res, req)
	})

	p.Get("/success", func(res http.ResponseWriter, req *http.Request) {

		// Check if user is already authenticated
		session, err := store.Get(req, "session-name")
		//fmt.Println(session)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		if auth, ok := session.Values["authenticated"].(bool); ok && auth {
			t, _ := template.ParseFiles("templates/success.html")
			// Lets make this an associative array where user is the key
			associativeArray := map[string]string{"userEmail": session.Values["userEmail"].(string)}

			//userEmail := session.Values["userEmail"]
			t.Execute(res, associativeArray)
		} else {
			//fmt.Println("User is not authenticated, redirecting to home page")
			http.Redirect(res, req, "/", http.StatusSeeOther)
		}
	})

	p.Get("/", func(res http.ResponseWriter, req *http.Request) {

		// Check if user is already authenticated
		session, err := store.Get(req, "session-name")
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		if auth, ok := session.Values["authenticated"].(bool); ok && auth {
			//fmt.Println("User is already authenticated, redirecting to success page")
			// Redirect to success page
			//http.Redirect(res, req, "/success", http.StatusSeeOther)
			//return
		}

		t, _ := template.ParseFiles("templates/index.html")
		t.Execute(res, false)
	})

	log.Println("listening on localhost:80")
	log.Fatal(http.ListenAndServe(":80", p))
}
