FROM golang:alpine3.11
LABEL rpoxy.routers.container.host="httpserver"
COPY httpserver ./httpserver
ENTRYPOINT [ "./httpserver" ]