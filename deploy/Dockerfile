# syntax=docker/dockerfile:1
FROM --platform=linux/amd64 golang:1.21-alpine3.19 as build

# git needed for compile time variable
RUN apk add --no-cache make git build-base

ARG GCP_PROJECT "unstable-build-blue-dev"

WORKDIR /src

COPY . .

RUN make release && cp target/linux_amd64/bluebot /bin/bluebot

# use clean image with no deps or install
FROM --platform=linux/amd64 alpine:latest
COPY --from=build /bin/bluebot /bin/bluebot

EXPOSE 8080

CMD [ "sh", "-c", "/bin/bluebot -p ${GCP_PROJECT} -J -C ${SLACK_CHANNEL_ID}" ]
