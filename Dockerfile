FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/updtr .

FROM golang:1.23-alpine
RUN apk add --no-cache ca-certificates git

COPY --from=build /out/updtr /usr/local/bin/updtr

ENTRYPOINT ["/usr/local/bin/updtr", "action"]
