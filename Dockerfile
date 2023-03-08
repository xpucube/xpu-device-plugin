FROM golang:1.10-stretch as build

WORKDIR /go/src/github.com/YoYoContainerService/xpu-device-plugin
COPY . .

RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go build -ldflags="-s -w" -o /go/bin/xpu-device-plugin-v2 cmd/nvidia/main.go

RUN go build -o /go/bin/kubectl-list-xpuinfo-v2 cmd/inspect/*.go

FROM debian:bullseye-slim

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

COPY --from=build /go/bin/xpu-device-plugin-v2 /usr/bin/xpu-device-plugin-v2

COPY --from=build /go/bin/kubectl-list-xpuinfo-v2 /usr/bin/kubectl-list-xpuinfo-v2

CMD ["xpu-device-plugin-v2","-logtostderr"]
