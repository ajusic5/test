package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/go-sql-driver/mysql"
)

type User struct {
	ID    int
	Name  string
	Email string
	Age   int
}

func getUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
    params := mux.Vars(r)
    id := params["id"]

    var user User
    err := db.QueryRow("SELECT Name, Email, Age FROM User WHERE ID = ?", id).Scan(&user.Name, &user.Email, &user.Age)
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    fmt.Fprintf(w, "Name: %s, Email: %s, Age: %d", user.Name, user.Email, user.Age)
}

func getUsers(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8") // Ensure UTF-8 encoding
	rows, err := db.Query("SELECT Name, Email, Age FROM User") 
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var result string
	for rows.Next() {
		var name, email string
		var age int
		if err := rows.Scan(&name, &email, &age); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		result += fmt.Sprintf("%s - %s - %d\n", name, email, age)
	}

	if result == "" {
		result = "No users found.\n"
	}
	w.Write([]byte(result))
}

func createUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var u User
	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO User (Name, Email, Age) VALUES (?, ?, ?)", u.Name, u.Email, u.Age)
	if err != nil {
		http.Error(w, "Error inserting user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func updateUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Retrieve existing user data
	var existing User
	err := db.QueryRow("SELECT Name, Email, Age FROM User WHERE ID=?", id).Scan(&existing.Name, &existing.Email, &existing.Age)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Decode the request body
	var updates User
	err = json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Use existing values if a field is not provided in the request
	if updates.Name == "" {
		updates.Name = existing.Name
	}
	if updates.Email == "" {
		updates.Email = existing.Email
	}
	if updates.Age == 0 { // Assuming Age 0 means it was not provided
		updates.Age = existing.Age
	}

	// Update the user in the database
	_, err = db.Exec("UPDATE User SET Name=?, Email=?, Age=? WHERE ID=?", updates.Name, updates.Email, updates.Age, id)
	if err != nil {
		http.Error(w, "Error updating user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := db.Exec("DELETE FROM User WHERE ID=?", id)
	if err != nil {
		http.Error(w, "Error deleting user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleRequests(db *sql.DB) {
	r := mux.NewRouter()

	r.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			getUsers(w, r, db)
		} else if r.Method == "POST" {
			createUser(w, r, db)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	r.HandleFunc("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET"{
			getUser(w, r, db)
		} else if r.Method == "PUT" {
			updateUser(w, r, db)
		} else if r.Method == "DELETE" {
			deleteUser(w, r, db)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func initializeDB() (*sql.DB, error) {
	// Connect as root to create the local database and user
	rootDsn := "root:@tcp(127.0.0.1:3306)/"
	db, err := sql.Open("mysql", rootDsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Create the database if it doesn't exist
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS go_db")
	if err != nil {
		return nil, fmt.Errorf("error creating database: %w", err)
	}

	// Create a new user if it doesn't exist
	newUser := "go_user"
	newPassword := "password"
	_, err = db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'", newUser, newPassword))
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	// Grant all privileges on go_db to the new user
	_, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON go_db.* TO '%s'@'localhost'", newUser))
	if err != nil {
		return nil, fmt.Errorf("error granting privileges: %w", err)
	}

	// Apply the changes in privileges
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return nil, fmt.Errorf("error flushing privileges: %w", err)
	}

	fmt.Println("Database and user successfully created!")

	// Connect using the new user
	userDsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/go_db", newUser, newPassword)
	db, err = sql.Open("mysql", userDsn)
	if err != nil {
		return nil, fmt.Errorf("error connecting with new user: %w", err)
	}

	// Create the User table if it doesn't exist
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS User (
		ID INT PRIMARY KEY AUTO_INCREMENT,
		Name VARCHAR(100) NOT NULL,
		Email VARCHAR(100) UNIQUE NOT NULL,
		Age INT
	);`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	// Insert test data if the table has no records
	_, err = db.Exec("INSERT INTO User (Name, Email, Age) SELECT * FROM (SELECT 'Amna Jusić', 'ajusic5@etf.unsa.ba', 25) AS tmp WHERE NOT EXISTS (SELECT * FROM User)")
	if err != nil {
		return nil, fmt.Errorf("error inserting data: %w", err)
	}

	return db, nil
}

func main() {
	//Connect to the database
	db, err := initializeDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Start the server
	handleRequests(db)
	
}
