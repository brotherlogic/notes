# ==========================================
# STAGE 1: Build the React + Vite Frontend
# ==========================================
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

# Copy package configurations and install dependencies
COPY frontend/package.json ./
RUN npm install

# Copy frontend source files and run the build compiler
COPY frontend/ ./
RUN npm run build

# ==========================================
# STAGE 2: Build the Go Backend Server
# ==========================================
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app

# Install system dependencies
RUN apk add --no-cache git ca-certificates

# Copy dependencies lists and cache them
COPY go.mod go.sum ./
RUN go mod download

# Copy source trees
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY proto/ ./proto/

# Statically compile the Go production binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o notes-server ./cmd/notes-server

# ==========================================
# STAGE 3: Final Lightweight Production Image
# ==========================================
FROM alpine:latest
WORKDIR /app

# Install runtime security certificates
RUN apk add --no-cache ca-certificates

# Pull Go binary from builder stage
COPY --from=backend-builder /app/notes-server .

# Pull static frontend production assets from compiler stage
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Set default production environment fallbacks
ENV PORT=8080
ENV DATA_DIR=/data/binaries
ENV FRONTEND_DIR=/app/frontend/dist

# Expose container socket
EXPOSE 8080

# Execute production server
ENTRYPOINT ["./notes-server"]
