FROM golang:1.27-alpine AS build
WORKDIR /src

# "direct" is the fallback when the module proxy is unreachable, and it needs git.
RUN apk add --no-cache git
ENV GOPROXY="https://proxy.golang.org|direct"

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/server ./cmd/migrate ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /out/migrate /out/seed /app/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
