FROM golang:1.16-alpine3.15 as disco-build
RUN apk add --no-cache git
ADD . /go/src/github.com/m-lab/disco
RUN cd /go/src/github.com/m-lab/disco && ./build.sh

# Now copy the built image into the minimal base image
FROM alpine:3.15
COPY --from=disco-build /go/bin/disco /
WORKDIR /
ENTRYPOINT ["/disco"]
