package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"runtime"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Define the struct to map to the user_data table
type UserData struct {
	ID         int    `gorm:"primaryKey;autoIncrement"`
	FirstName  string `gorm:"size:100"`
	LastName   string `gorm:"size:100"`
	Email      string `gorm:"size:150"`
	Age        int
	Gender     string `gorm:"size:10"`
	Department string `gorm:"size:100"`
	Company    string `gorm:"size:100"`
	Salary     float64
	DateJoined string `gorm:"type:date"`
	IsActive   bool
}

// TableName specifies the name of the table in the database
func (UserData) TableName() string {
	return "user_data"
}

// DBHandler interface defines methods for database operations
type DBHandler interface {
	Find(dest interface{}, conds ...interface{}) *gorm.DB
	Offset(offset int) DBHandler
	Limit(limit int) DBHandler
	Order(value string) DBHandler
	CreateInBatches(value interface{}, batchSize int) error // Change return type to error
}

// GormDBHandler is a concrete implementation of DBHandler using GORM
type GormDBHandler struct {
	db *gorm.DB
}

// Implement the Find method for GormDBHandler
func (handler *GormDBHandler) Find(dest interface{}, conds ...interface{}) *gorm.DB {
	return handler.db.Find(dest, conds...)
}

// Implement the Offset method for GormDBHandler
func (handler *GormDBHandler) Offset(offset int) DBHandler {
	handler.db = handler.db.Offset(offset)
	return handler
}

// Implement the Limit method for GormDBHandler
func (handler *GormDBHandler) Limit(limit int) DBHandler {
	handler.db = handler.db.Limit(limit)
	return handler
}

// Implement the Order method for GormDBHandler
func (handler *GormDBHandler) Order(value string) DBHandler {
	handler.db = handler.db.Order(value)
	return handler
}

// Implement the CreateInBatches method to match the DBHandler interface
func (handler *GormDBHandler) CreateInBatches(value interface{}, batchSize int) error {
	// The value is an interface{} here, so we need to type assert it to []UserData
	users, ok := value.([]UserData)
	if !ok {
		return fmt.Errorf("expected []UserData but got %T", value)
	}

	// Perform batch creation
	return handler.db.CreateInBatches(users, batchSize).Error
}

// Initialize PostgreSQL connection using GORM
func setupDatabase() *gorm.DB {
	dsn := "host=localhost user=postgres password=Virat@2#Virat@2# dbname=mini-Project port=8899 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to the database: " + err.Error())
	}

	// Automatically create the "user_data" table if it doesn't exist
	if err := db.AutoMigrate(&UserData{}); err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	fmt.Println("Database connected and table user_data created successfully.")
	return db
}

// Log memory usage
func logMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Memory Usage: Allocated: %d KB, Total Alloc: %d KB, System: %d KB\n",
		m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024)
}

// Read CSV in chunks and send data to a channel
func readCSVChunk(file multipart.File, chunkSize int, ch chan<- [][]string) {
	reader := csv.NewReader(bufio.NewReader(file))

	_, _ = reader.Read() // Skip the header row

	for {
		records := make([][]string, 0, chunkSize)
		for i := 0; i < chunkSize; i++ {
			record, err := reader.Read()
			if err != nil {
				if err == io.EOF {
					if len(records) > 0 {
						ch <- records // Send the last chunk
					}
					close(ch)
					return
				}
				fmt.Printf("Error reading CSV file: %v\n", err)
				close(ch)
				return
			}
			records = append(records, record)
		}
		ch <- records
	}
}

// Process a chunk of CSV records and store them in the database
func processChunk(records [][]string, dbHandler DBHandler, batchSize int, semaphore chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	// Acquire semaphore
	semaphore <- struct{}{}

	// Declare the array of users that will be inserted
	var users []UserData
	for _, record := range records {
		// Parse record values safely
		age, err := strconv.Atoi(record[4])
		if err != nil {
			fmt.Printf("Skipping record with invalid age: %v\n", record)
			continue // Skip invalid records
		}

		salary, err := strconv.ParseFloat(record[8], 64)
		if err != nil {
			fmt.Printf("Skipping record with invalid salary: %v\n", record)
			continue // Skip invalid records
		}

		isActive := record[10] == "true"

		// Construct UserData object
		users = append(users, UserData{
			FirstName:  record[1],
			LastName:   record[2],
			Email:      record[3],
			Age:        age,
			Gender:     record[5],
			Department: record[6],
			Company:    record[7],
			Salary:     salary,
			DateJoined: record[9],
			IsActive:   isActive,
		})
	}

	// Batch insert
	if len(users) > 0 {
		if err := dbHandler.CreateInBatches(users, batchSize); err != nil {
			fmt.Printf("Database insertion error: %v\n", err)
		}
	}

	// Free up memory and trigger garbage collection
	runtime.GC()

	// Release semaphore
	<-semaphore
}

// POST handler for CSV file upload
func uploadCSV(c *gin.Context, dbHandler DBHandler) {
	// Get file from form-data
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to get file", "details": err.Error()})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to open file", "details": err.Error()})
		return
	}
	defer file.Close()

	// Initialize CSV processing
	chunkSize := 5000               // Adjust chunk size
	batchSize := 10000              // Set batch size to stay within parameter limit
	ch := make(chan [][]string, 10) // Increase buffered channel size for better performance
	var wg sync.WaitGroup

	// Semaphore to limit the number of concurrent Goroutines
	semaphore := make(chan struct{}, runtime.NumCPU()*4)

	// Start reading the CSV file in chunks
	go readCSVChunk(file, chunkSize, ch)

	// Process each chunk in a separate Goroutine
	for records := range ch {
		wg.Add(1)

		go processChunk(records, dbHandler, batchSize, semaphore, &wg)

		// Optional: Log memory usage
		logMemoryUsage() // This can be enabled for debugging
	}

	// Wait for all Goroutines to finish
	wg.Wait()

	// Respond with success message
	c.JSON(200, gin.H{"message": "CSV file processed successfully and data stored in database."})
}

func CSVtoDB() {
	// Set up the database using the updated setupDatabase function
	db := setupDatabase()

	// Create a GormDBHandler instance that implements the DBHandler interface
	var dbHandler DBHandler = &GormDBHandler{db: db}

	// Create a new Gin router
	r := gin.Default()
	r.MaxMultipartMemory = 30 << 30 // 30 GB for large file uploads

	// Define the POST endpoint to upload the CSV file
	r.POST("/upload-csv", func(c *gin.Context) {
		uploadCSV(c, dbHandler)
	})

	// Start the Gin server
	r.Run(":8080")
}
