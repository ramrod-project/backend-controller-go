FROM alpine:3.7

RUN apk update \
    && apk add go1.10=1.10-r0

WORKDIR /run

COPY ./controller .
COPY ./manifest.json .

CMD ["./controller"]