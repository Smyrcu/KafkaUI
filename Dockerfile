FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./internal/frontend/dist
RUN rm -f ./internal/frontend/dist/.gitkeep
RUN CGO_ENABLED=0 go build -o kafkaui ./cmd/kafkaui

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=backend /app/kafkaui /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["kafkaui"]
CMD ["--config", "/etc/kafkaui/config.yaml"]
