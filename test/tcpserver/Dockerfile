FROM golang:alpine3.11
LABEL rpoxy.routers.container.host="tcpserver"
COPY tcpserver ./tcpserver
ENTRYPOINT [ "./tcpserver" ]