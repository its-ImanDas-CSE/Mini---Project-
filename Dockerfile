# Start with a Go base image
FROM golang:1.23.3-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum are not changed
RUN go mod tidy

# Copy the source code into the container
COPY . .

# Build the Go app
RUN go build -o user_data_api .

# Start a new stage from a smaller Alpine image
FROM alpine:latest

# Install required packages in the smaller image (for running the app)
RUN apk --no-cache add ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/user_data_api .

# Expose port 8080 for the API
EXPOSE 8080

# Command to run the executable
CMD ["C:\Users\iman.das\Desktop\GoLang\Mini-Project\user_data_API.go"]
