FROM --platform=$BUILDPLATFORM golang:1.27-trixie AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ado-mcp ./cmd/ado-mcp

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /out/ado-mcp /ado-mcp
EXPOSE 8888
ENTRYPOINT ["/ado-mcp", "--host", "0.0.0.0"]
