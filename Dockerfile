FROM golang:1.25-alpine AS build
WORKDIR /app
COPY main.go .
RUN go build -o podmeter main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=build /app/podmeter .
EXPOSE 8080
CMD ["./podmeter"]