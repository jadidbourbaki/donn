# Build a static binary, then run it on a distroless base.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /donn ./cmd/donn

FROM gcr.io/distroless/static-debian12
COPY --from=build /donn /donn
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/donn"]
