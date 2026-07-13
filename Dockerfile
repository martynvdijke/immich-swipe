# Stage 1: Build frontend
FROM node:24-alpine AS frontend-builder

WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Stage 2: Build Go server
FROM golang:1.26-alpine AS server-builder

WORKDIR /server
COPY --from=frontend-builder /app/package.json .
COPY server/ .
RUN VERSION=$(grep -o '"version": *"[^"]*"' package.json | cut -d'"' -f4) && \
    CGO_ENABLED=0 go build -ldflags="-X main.Version=$VERSION" -o immich-swipe-server .

# Stage 3: Runtime
FROM alpine:3.24

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=frontend-builder /app/dist ./dist
COPY --from=server-builder /server/immich-swipe-server .

EXPOSE 8080

ENV LISTEN_ADDR=:8080
ENV STATIC_DIR=./dist

CMD ["./immich-swipe-server"]
