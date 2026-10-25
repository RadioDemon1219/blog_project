FROM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /src/blog .

FROM alpine:3.20
WORKDIR /app

COPY --from=build /src/blog /app/blog
COPY --from=build /src/templates ./templates

EXPOSE 8080
CMD ["/app/blog"]