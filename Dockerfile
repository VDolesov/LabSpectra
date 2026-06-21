FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /labspectra .

FROM gcr.io/distroless/static-debian12
COPY --from=build /labspectra /labspectra
ENV DATA_DIR=/data
ENTRYPOINT ["/labspectra", "-no-open"]
