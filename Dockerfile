FROM alpine
WORKDIR /
COPY /bin/* ./
COPY /migrations /migrations

CMD ["/microservice"]