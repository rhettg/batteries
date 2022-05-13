FROM golang:1.18

WORKDIR /usr/src/yakapi

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/yakapi .

ENV YAKAPI_PORT=8080
ENV YAKAPI_NAME="Yak Bot"
ENV YAKAPI_PROJECT_URL="https://github.com/The-Yak-Collective/yakrover"
ENV YAKAPI_ADAPTER_MOTOR="echo"

CMD ["yakapi"]