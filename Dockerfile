FROM golang:1.17.2-alpine AS build
WORKDIR /usr/src
RUN apk add --no-cache gcc=10.3.1_git20210424-r2 build-base=0.5-r2
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . ./
RUN go build -o app -ldflags "-s -w"

FROM alpine:3
WORKDIR /usr/src
COPY --from=build /usr/src/app /usr/src/
CMD ["/usr/src/app"]
