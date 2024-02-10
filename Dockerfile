# Use the official Go image as a parent image
FROM golang:1.21 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the local package files to the container's workspace
COPY . .

# Fetch application dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -v -o nimiq-activator ./cmd

# Use a Docker multi-stage build to create a lean production image
# Start with a new stage from scratch
FROM alpine:latest  

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/nimiq-activator .

# Command to run the executable
CMD ["./nimiq-activator"]
