package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/go-sql-driver/mysql"
)

type User struct {
	ID    int 
	Name  string 
	Email string 
	Age   int   
}

type UserResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func getUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")
    params := mux.Vars(r)
    id := params["id"]

    var user UserResponse
    err := db.QueryRow("SELECT Name, Email, Age FROM User WHERE ID = ?", id).Scan(&user.Name, &user.Email, &user.Age)
    if err != nil {
        if err == sql.ErrNoRows {
			http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		}
		return
    }

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

func getUsers(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")
	rows, err := db.Query("SELECT Name, Email, Age FROM User") 
	if err != nil {
		log.Println(err)
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users [] UserResponse
	for rows.Next() {
		var user UserResponse
		if err := rows.Scan(&user.Name, &user.Email, &user.Age); err != nil {
			http.Error(w, `{"error": "Error reading user data"}`, http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}

func isValidEmail(email string) bool {
	//Checking email structure
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

func createUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")

	if r.Body == nil || r.ContentLength == 0 {
		http.Error(w, `{"error": "Request body is empty"}`, http.StatusBadRequest)
		return
	}
	
	var u User

	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, `{"error": "Invalid input"}`, http.StatusBadRequest)
		return
	}

	if u.Name == "" || u.Email == "" {
		http.Error(w, `{"error": "Name and Email are required"}`, http.StatusBadRequest)
		return
	}

	if !isValidEmail(u.Email) {
		http.Error(w, `{"error": "Invalid email format"}`, http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO User (Name, Email, Age) VALUES (?, ?, ?)", u.Name, u.Email, u.Age)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 { // 1062 je kod za DUPLICATE ENTRY
				http.Error(w, `{"error": "Email is already in use"}`, http.StatusConflict)
				return
			}
		}
		http.Error(w, `{"error": "Error inserting user"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User successfully created"})
}

func updateUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	id := vars["id"]

	// Retrieve existing user data
	var existing User
	err := db.QueryRow("SELECT Name, Email, Age FROM User WHERE ID=?", id).Scan(&existing.Name, &existing.Email, &existing.Age)
	if err != nil {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	if r.Body == nil || r.ContentLength == 0{
		http.Error(w, `{"error": "Cannot edit user - Request body is required"}`, http.StatusBadRequest)
		return
	}

	// Decode the request body
	var updates User
	err = json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		http.Error(w, `{"error": "Invalid input"}`, http.StatusBadRequest)
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

	if !isValidEmail(updates.Email) {
		http.Error(w, `{"error": "Invalid email format"}`, http.StatusBadRequest)
		return
	}

	// Update the user in the database
	_, err = db.Exec("UPDATE User SET Name=?, Email=?, Age=? WHERE ID=?", updates.Name, updates.Email, updates.Age, id)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			switch mysqlErr.Number {
			case 1062: // Duplicate entry (email already in use)
				http.Error(w, `{"error": "Email is already in use"}`, http.StatusConflict)
				return
			case 1048: // Column cannot be null (invalid email format)
				http.Error(w, `{"error": "Invalid email format"}`, http.StatusBadRequest)
				return
			}
		}
		http.Error(w, `{"error": "Error updating user"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "User successfully updated"})
}

func deleteUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	id := vars["id"]

	result, err := db.Exec("DELETE FROM User WHERE ID=?", id)
	if err != nil {
		http.Error(w, `{"error": "Error deleting user"}`, http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, `{"error": "Error retrieving delete status"}`, http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "User successfully deleted"})
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
		return nil, fmt.Errorf("Error creating database: %w", err)
	}

	// Create a new user if it doesn't exist
	newUser := "go_user"
	newPassword := "password"
	_, err = db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'", newUser, newPassword))
	if err != nil {
		return nil, fmt.Errorf("Error creating user: %w", err)
	}

	// Grant all privileges on go_db to the new user
	_, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON go_db.* TO '%s'@'localhost'", newUser))
	if err != nil {
		return nil, fmt.Errorf("Error granting privileges: %w", err)
	}

	// Apply the changes in privileges
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return nil, fmt.Errorf("Error flushing privileges: %w", err)
	}

	fmt.Println("Database and user successfully created!")

	// Connect using the new user
	userDsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/go_db", newUser, newPassword)
	db, err = sql.Open("mysql", userDsn)
	if err != nil {
		return nil, fmt.Errorf("Error connecting with new user: %w", err)
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
		return nil, fmt.Errorf("Error creating table: %w", err)
	}

	// Insert test data if the table has no records
	_, err = db.Exec("INSERT INTO User (Name, Email, Age) SELECT * FROM (SELECT 'Amna Jusić', 'ajusic5@etf.unsa.ba', 25) AS tmp WHERE NOT EXISTS (SELECT * FROM User)")
	if err != nil {
		return nil, fmt.Errorf("Error inserting data: %w", err)
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
