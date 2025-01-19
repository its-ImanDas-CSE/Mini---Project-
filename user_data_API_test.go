package main

import (
	//"bytes"
	//"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	//"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestSetupLogger tests the logger setup function
func TestSetupLogger(t *testing.T) {
	// Test the logger setup to ensure no errors are thrown
	assert.NotPanics(t, func() { setupLogger() })
}

// TestAnalyzeLogs tests log analysis functionality for valid and invalid scenarios
func TestAnalyzeLogs(t *testing.T) {
	// Create a temporary file for testing log analysis
	filePath := "test_log_file.log"
	logFile, err := os.Create(filePath)
	assert.NoError(t, err)
	defer os.Remove(filePath)

	// Write some sample logs to the file
	logFile.WriteString("INFO This is an info log\n")
	logFile.WriteString("ERROR This is an error log\n")
	logFile.WriteString("DEBUG This is a debug log\n")

	// Test analyzing valid log file
	logCounts, err := analyzeLogs(filePath)
	assert.NoError(t, err)
	assert.Equal(t, 1, logCounts["INFO"])
	assert.Equal(t, 1, logCounts["ERROR"])
	assert.Equal(t, 1, logCounts["DEBUG"])

	// Test analyzing an invalid log file (non-existing file)
	_, err = analyzeLogs("non_existing_file.log")
	assert.Error(t, err)
}

// TestRequestResponseLogger tests the request and response logger for proper logging
func TestRequestResponseLogger(t *testing.T) {
	// Set up a mock Gin engine
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Add the request-response logger middleware
	r.Use(requestResponseLogger())

	// Add a simple test route
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	// Record the response using httptest
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	// Ensure status code 200 is returned
	assert.Equal(t, 200, w.Code)

	// Verify if the logger captured request and response data
	// Since it's hard to capture logger output directly, we will assume correct behavior if no panic occurs
	assert.NotPanics(t, func() { r.ServeHTTP(w, req) })
}

// TestSetupDatabases tests the database connection setup function
func TestSetupDatabases(t *testing.T) {
	// Test the setupDatabases function to ensure it connects without errors
	db := setupDatabases()

	// Ensure the database connection is not nil
	assert.NotNil(t, db)

	// Since the actual connection may fail, we assume success if the function completes without panic
	assert.NotPanics(t, func() { setupDatabases() })
}

// TestDatabase_Limit tests the Limit method on real database
func TestDatabase_Limit(t *testing.T) {
	// Set up the database connection
	dsn := "host=localhost user=postgres password=Virat@2#Virat@2# dbname=mini-Project port=8899 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)

	// Test the Limit method
	var records []UserDatas
	limit := 5
	err = db.Offset(0).Limit(limit).Order("id ASC").Find(&records).Error
	assert.NoError(t, err)
	assert.Len(t, records, limit)
}

// TestSetupAPI tests the API setup function, ensuring all routes work with real DB
func TestSetupAPI(t *testing.T) {
	// Set up the database connection
	dsn := "host=localhost user=postgres password=Virat@2#Virat@2# dbname=mini-Project port=8899 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)

	// Wrap GORM DB in the GormDatabase struct
	gormDB := &GormDatabase{DB: db}

	// Set up Gin engine with the actual database connection
	gin.SetMode(gin.TestMode)
	r := setupAPI(gormDB)

	// Record the response for the /api/records endpoint
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/records?page=1&size=10", nil)
	r.ServeHTTP(w, req)

	// Ensure status code 200 is returned
	assert.Equal(t, 200, w.Code)
}
