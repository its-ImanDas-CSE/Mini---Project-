package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// UserDatas defines the struct to map to the user_data table
type UserDatas struct {
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
func (UserDatas) TableName() string {
	return "user_data"
}

// Database interface for database operations
type Database interface {
	Find(dest interface{}, conds ...interface{}) *gorm.DB
	Offset(offset int) Database
	Limit(limit int) Database
	Order(value string) Database
}

// GormDatabase is the concrete implementation of the Database interface
type GormDatabase struct {
	DB *gorm.DB
}

// Implement the Database interface for GormDatabase
func (g *GormDatabase) Find(dest interface{}, conds ...interface{}) *gorm.DB {
	return g.DB.Find(dest, conds...)
}

func (g *GormDatabase) Offset(offset int) Database {
	g.DB = g.DB.Offset(offset)
	return g
}

func (g *GormDatabase) Limit(limit int) Database {
	g.DB = g.DB.Limit(limit)
	return g
}

func (g *GormDatabase) Order(value string) Database {
	g.DB = g.DB.Order(value)
	return g
}

// Initialize Logrus logger
var log = logrus.New()

// setupLogger configures Logrus with log rotation
func setupLogger() {
	log.SetOutput(&lumberjack.Logger{
		Filename:   "File.log",
		MaxSize:    10,   // Max size in MB before rotating
		MaxBackups: 3,    // Max number of old log files to keep
		MaxAge:     7,    // Max age in days to keep old log files
		Compress:   true, // Compress old log files
	})
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)
}

// setupDatabases initializes PostgreSQL connection using GORM
func setupDatabases() *gorm.DB {
	dsn := "host=localhost user=postgres password=Virat@2#Virat@2# dbname=mini-Project port=8899 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to the database")
	}
	log.Info("Successfully connected to the database")

	// Migrate the schema to create the table if it doesn't exist
	db.AutoMigrate(&UserDatas{})

	return db
}

// analyzeLogs analyzes the log file and counts the occurrences of different log levels
func analyzeLogs(filePath string) (map[string]int, error) {
	logCounts := map[string]int{"INFO": 0, "ERROR": 0, "DEBUG": 0}

	// Check if the log file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.WithField("filePath", filePath).Error("Log file does not exist")
		return nil, fmt.Errorf("log file does not exist: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.WithError(err).WithField("filePath", filePath).Error("Failed to open log file")
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Increase the scanner buffer size to handle large log lines
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 10*1024*1024) // 10 MB buffer
	scanner.Buffer(buf, 10*1024*1024)

	// Read the log file line by line
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.ToUpper(line) // Handle case-insensitivity
		if strings.Contains(line, "INFO") {
			logCounts["INFO"]++
		} else if strings.Contains(line, "ERROR") {
			logCounts["ERROR"]++
		} else if strings.Contains(line, "DEBUG") {
			logCounts["DEBUG"]++
		}
	}

	if err := scanner.Err(); err != nil {
		log.WithError(err).Error("Failed to read log file")
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	log.WithField("logCounts", logCounts).Info("Log analysis completed")
	return logCounts, nil
}

// requestResponseLogger logs incoming requests and outgoing responses
func requestResponseLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Log request details (method and URL only, no body)
		log.WithFields(logrus.Fields{
			"method": c.Request.Method,
			"url":    c.Request.URL.String(),
		}).Info("Incoming request")

		// Capture the response
		responseWriter := &responseCapture{ResponseWriter: c.Writer, body: new(bytes.Buffer)}
		c.Writer = responseWriter

		c.Next() // Process the request

		// Log response metadata (status and duration only)
		duration := time.Since(startTime)
		log.WithFields(logrus.Fields{
			"status":   responseWriter.Status(),
			"duration": duration.String(),
		}).Info("Outgoing response")
	}
}

// responseCapture captures the response body and status code
type responseCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseCapture) Write(data []byte) (int, error) {
	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}

// setupAPI sets up the API with REST endpoints using Gin
func setupAPI(db Database) *gin.Engine {
	r := gin.New()
	r.Use(requestResponseLogger())

	// Endpoint to retrieve all user records from the database
	r.GET("/api/records", func(c *gin.Context) {
		pageStr := c.DefaultQuery("page", "1")
		sizeStr := c.DefaultQuery("size", "10")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			log.WithField("page", pageStr).Error("Invalid page number")
			c.JSON(400, gin.H{"error": "Invalid page number"})
			return
		}

		size, err := strconv.Atoi(sizeStr)
		if err != nil || size < 1 {
			log.WithField("size", sizeStr).Error("Invalid size number")
			c.JSON(400, gin.H{"error": "Invalid size number"})
			return
		}

		offset := (page - 1) * size
		var records []UserDatas

		if err := db.Offset(offset).Limit(size).Order("id ASC").Find(&records).Error; err != nil {
			log.WithError(err).Error("Failed to fetch records")
			c.JSON(500, gin.H{"error": "Failed to fetch records"})
			return
		}

		log.WithField("records_count", len(records)).Info("Records fetched successfully")
		c.JSON(200, records)
	})

	// Endpoint to retrieve analyzed logs
	r.GET("/api/logs", func(c *gin.Context) {
		logCounts, err := analyzeLogs("File.log")
		if err != nil {
			log.WithError(err).Error("Failed to analyze logs")
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, logCounts)
	})

	return r
}

func main() {
	// Set up the logger
	setupLogger()

	// Set up the database
	db := setupDatabases()

	// Wrap GORM DB in the interface implementation
	gormDB := &GormDatabase{DB: db}

	// Set up API with the Database interface
	r := setupAPI(gormDB)

	// Run the API on port 8080
	log.Info("Starting server on port 8080")
	if err := r.Run(":8080"); err != nil {
		log.WithError(err).Fatal("Failed to start the server")
	}
}
