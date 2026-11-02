FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/httpserver/dist ./internal/httpserver/dist
RUN CGO_ENABLED=0 go build -o /launchcrew ./cmd/launchcrew

FROM gcr.io/distroless/static-debian12
COPY --from=build /launchcrew /launchcrew
EXPOSE 8080
ENTRYPOINT ["/launchcrew"]
