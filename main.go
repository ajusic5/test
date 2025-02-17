package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"io"
	pb "github.com/ajusic5/test/proto"
	"github.com/gorilla/mux"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/protobuf/encoding/protojson"
	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Address string `mapstructure:"address"`
		Port    int    `mapstructure:"port"`
	} `mapstructure:"server"`
	Database struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
		User string `mapstructure:"user"`
		Pass string `mapstructure:"pass"`
		Name string `mapstructure:"name"`
	} `mapstructure:"database"`
}


func getUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")
    params := mux.Vars(r)
    id := params["id"]

    var user pb.User
    err := db.QueryRow("SELECT Name, Email, Age FROM User WHERE ID = ?", id).Scan(&user.Name, &user.Email, &user.Age)
    if err != nil {
        if err == sql.ErrNoRows {
			http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		}
		return
    }

	// Converting protobuf object to JSON
	response, err := protojson.Marshal(&user)
	if err != nil {
		http.Error(w, `{"error": "Failed to serialize user"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func getUsers(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")

	// Get query parameters for pagination
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = 2 // Default value
	}

	//Calculate the offset
	offset := (page - 1) * limit

	// Query with pagination
	rows, err := db.Query("SELECT Name, Email, Age FROM User LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := &pb.UserList{}

	for rows.Next() {
		user := pb.User{}
		if err := rows.Scan(&user.Name, &user.Email, &user.Age); err != nil {
			http.Error(w, `{"error": "Error reading user data"}`, http.StatusInternalServerError)
			return
		}
		users.Users = append(users.Users, &user)
	}

	// Check the total number of users to calculate total pages
	var totalUsers int
	err = db.QueryRow("SELECT COUNT(*) FROM User").Scan(&totalUsers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response, err := protojson.Marshal(users)
	if err != nil {
		http.Error(w, `{"error": "Failed to encode users"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
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

	//read body into []byte
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}
	
	var user pb.User

	// Decode JSON to protobuf
	err = protojson.Unmarshal(body, &user)
	if err != nil {
		http.Error(w, `{"error": "Invalid input"}`, http.StatusBadRequest)
		return
	}

	if user.Name == "" || user.Email == "" {
		http.Error(w, `{"error": "Name and Email are required"}`, http.StatusBadRequest)
		return
	}

	if !isValidEmail(user.Email) {
		http.Error(w, `{"error": "Invalid email format"}`, http.StatusBadRequest)
		return
	}

	var res sql.Result
	res, err = db.Exec("INSERT INTO User (Name, Email, Age) VALUES (?, ?, ?)", user.Name, user.Email, user.Age)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 { // 1062 Duplicate entry error
				http.Error(w, `{"error": "Email is already in use"}`, http.StatusConflict)
				return
			}
		}
		http.Error(w, `{"error": "Error inserting user"}`, http.StatusInternalServerError)
		return
	}

	// Get inserted user ID
	 lastInsertID, err := res.LastInsertId()
	if err == nil {
		user.Id = int32(lastInsertID)
	} 
	
	responseData := struct {
		Message string  `json:"message"`
		User    *pb.User `json:"user"`
	}{
		Message: "User successfully created",
		User:    &user,
	}

	// Encode response 
	response, err := json.Marshal(responseData)
	if err != nil {
		http.Error(w, `{"error": "Error encoding response"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(response)
}

func updateUser(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	id := vars["id"]

	// Retrieve existing user data
	var existing pb.User
	err := db.QueryRow("SELECT Name, Email, Age FROM User WHERE ID=?", id).Scan(&existing.Name, &existing.Email, &existing.Age)
	if err != nil {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	if r.Body == nil || r.ContentLength == 0{
		http.Error(w, `{"error": "Cannot edit user - Request body is required"}`, http.StatusBadRequest)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}

	// Decode the request body
	var updates pb.User
	err = protojson.Unmarshal(body, &updates)
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

	responseData := struct {
		Message string  `json:"message"`
		User    *pb.User `json:"user"`
	}{
		Message: "User successfully updated",
		User:    &updates,
	}

	// Return updated user
	response, err := json.Marshal(responseData)
	if err != nil {
		http.Error(w, `{"error": "Error encoding response"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
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

	responseData := map[string]string{"message": "User successfully deleted"}
	response, err := json.Marshal(responseData)
	if err != nil {
    	http.Error(w, `{"error": "Error encoding response"}`, http.StatusInternalServerError)
    	return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func handleRequests(db *sql.DB, address string, port int) {
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

	serverAddress := fmt.Sprintf("%s:%d", address, port)
	fmt.Printf("Server is running on %s\n", serverAddress)
	log.Fatal(http.ListenAndServe(serverAddress, r))
}

func initializeDB() (*sql.DB, error) {
	// Connect as root to create the local database and user
	rootDsn := "root:password@tcp(localhost:3306)/"
	db, err := sql.Open("mysql", rootDsn)
	if err != nil {
		return nil, err
	}

	// Create the database if it doesn't exist 
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS go_db")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Error creating database: %w", err)
	}

	// Create a new user if it doesn't exist
	newUser := "go_user"
	newPassword := "password"
	_, err = db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'", newUser, newPassword))
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Error creating user: %w", err)
	}

	// Grant all privileges on go_db to the new user
	_, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON go_db.* TO '%s'@'localhost'", newUser))
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Error granting privileges: %w", err)
	}

	// Apply the changes in privileges
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Error flushing privileges: %w", err)
	}

	fmt.Println("Database and user successfully created!")

	// Close the root connection
	db.Close();

	// Connect using the new user
	 userDsn := fmt.Sprintf("%s:%s@tcp(localhost:3306)/go_db", newUser, newPassword)
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
		db.Close()
		return nil, fmt.Errorf("Error creating table: %w", err)
	}

	// Insert test data if the table has no records
	_, err = db.Exec("INSERT INTO User (Name, Email, Age) SELECT * FROM (SELECT 'Amna Jusić', 'ajusic5@etf.unsa.ba', 25) AS tmp WHERE NOT EXISTS (SELECT * FROM User)")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Error inserting data: %w", err)
	}

	return db, nil
}

func connectDB(config Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", config.Database.User, config.Database.Pass, config.Database.Host, config.Database.Port, config.Database.Name)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Check database connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func loadConfig() (Config, error) {
	var config Config
	viper.SetConfigFile("config.yaml")

	if err := viper.ReadInConfig(); err != nil {
		return config, err
	}

	err := viper.Unmarshal(&config)
	if err != nil {
		return config, err
	}

	return config, nil
}

func main() {

	// Configuration loading
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Connect to a database
	db, err := connectDB(config)
	if err != nil {
		// First time run
		db, err = initializeDB()
		if err != nil {
			log.Fatal(err)
		}
	}
	defer db.Close()
	handleRequests(db, config.Server.Address, config.Server.Port)
}
