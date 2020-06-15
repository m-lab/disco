FROM golang:1.14-alpine3.12 as disco-build
RUN apk add --no-cache git
ADD . /go/src/github.com/m-lab/disco
RUN cd /go/src/github.com/m-lab/disco && ./build.sh

# Now copy the built image into the minimal base image
FROM alpine:3.12
COPY --from=disco-build /go/bin/disco /
WORKDIR /
ENTRYPOINT ["/disco"]
