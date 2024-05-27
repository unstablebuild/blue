# syntax=docker/dockerfile:1
FROM --platform=linux/amd64 golang:alpine as build

# git and openssh needed to go get private repos
RUN apk add --no-cache make build-base

ARG GCP_PROJECT "unstable-build-blue-dev"

WORKDIR /src

COPY . .

RUN make release && cp target/linux_amd64/bluebot /bin/bluebot

# use clean image with no deps or install
FROM --platform=linux/amd64 alpine:latest
COPY --from=build /bin/bluebot /bin/bluebot

CMD [ "sh", "-c", "/bin/bluebot -p ${GCP_PROJECT} -J -C ${SLACK_CHANNEL_ID}" ]
