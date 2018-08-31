FROM alpine:3.7

WORKDIR /run

COPY ./controller .
COPY ./manifest.json .

CMD ["./controller"]