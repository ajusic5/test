# Test Go Project
## Prerequisites
Ensure you have the following installed:
- Git
- Go
- MySQL
  - A running MySQL server
  - A MySQL root user with password `password` (used only for initial database and user creation)   
- cURL (optional - for testing via cmd)
## Setup instructions
### Clone the repository
`git clone https://github.com/ajusic5/test.git folder-name`

`cd folder-name`
## Configuration
### Database configuration
- Ensure MySQL is running on the default port
- The application will automatically create the necessary database and user on the first run. Just make sure that MySQL has a root user with:
  - Username: `root`
  - Password: `password`
### Application configuration
- The application runs on port `8080`
- If needed, you can change the configuration data in the `config.yaml` file
- If you change database settings, you must also update the `InitializeDB` and `ConnectDB` functions in `main.go`
## Run the application
`go mod tidy`

`go run main.go`

## Testing 
[Backend demo and tests example file](https://drive.google.com/drive/folders/1wSclrm6c0YloYDQTmWj8IMN4J4I8AQqz?usp=sharing)



  
  
