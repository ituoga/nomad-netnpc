FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 go build -o /npc


FROM scratch
WORKDIR /
COPY --from=0 /npc /

EXPOSE 80

ENTRYPOINT ["/npc"]