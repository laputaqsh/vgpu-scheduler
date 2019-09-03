FROM golang:1.12.7 as build

WORKDIR /go/src/vgpu-scheduler

COPY . .

RUN go build -o /go/bin/vgpu-scheduler

FROM debian:stretch-slim

COPY --from=build /go/bin/vgpu-scheduler /usr/bin/vgpu-scheduler

ENTRYPOINT ["vgpu-scheduler"]
