FROM golang:1.24

ARG BUILD_TIMESTAMP

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make

ENV BUILD_TIMESTAMP=${BUILD_TIMESTAMP}
CMD ["./bin/teobot", "server"]
