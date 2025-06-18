# --- Build Stage ---
# Use an official Go runtime as a parent image for building the application
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files to leverage Docker cache
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code from the current directory to the working directory inside the container
COPY . .

# Build the Go app as a static binary. This is crucial for a small final image size.
# CGO_ENABLED=0 disables CGO
# GOOS=linux specifies the target operating system
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o k8s-metrics-app .

# --- Final Stage ---
# Use a minimal base image for the final container to keep it lightweight
FROM alpine:latest

# Set the working directory for the final image
WORKDIR /root/

# Copy the static frontend file from the build context into the final image
COPY index.html .

# Copy the pre-built binary from the "builder" stage
COPY --from=builder /app/k8s-metrics-app .

# Expose port 8080 to the outside world, this is the port our Go app listens on
EXPOSE 8080

# Command to run the executable when the container starts
CMD ["./k8s-metrics-app"]
